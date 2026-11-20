package data

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/model"
)

var (
	ErrInvalidPageToken      = errors.New("invalid page token")
	ErrModuleNotFound        = errors.New("module not found")
	ErrTagAlreadyExists      = errors.New("tag already exists")
	ErrTagNotFound           = errors.New("tag not found")
	ErrDependencyTagNotFound = errors.New("dependency tag not found")
)

const maxModuleTags = 500

func scopedUserIDFromContext(ctx context.Context) *uuid.UUID {
	if !model.IsAdmin(ctx) {
		if user, ok := model.GetUserFromContext(ctx); ok && user != nil {
			return &user.ID
		}
	}
	return nil
}

type RegistryRepository interface {
	RegisterModule(ctx context.Context, moduleName string) error
	GetModule(ctx context.Context, name string) (*v1.Module, error)
	GetModuleWithDrafts(ctx context.Context, name string, includeDraftTags bool) (*v1.Module, error)
	ListModules(ctx context.Context, pageSize int, token string) ([]*v1.Module, string, error)
	DeleteModule(ctx context.Context, name string) error
	PushModule(ctx context.Context, name string, tag string, protofiles []*v1.ProtoFile, dependencies ...*v1.Dependency) (*v1.Module, error)
	PushDraftModule(ctx context.Context, name string, tag string, protofiles []*v1.ProtoFile, dependencies []*v1.Dependency) (*v1.Module, error)
	PullModule(ctx context.Context, name string, tag string) (*v1.Module, []*v1.ProtoFile, error)
	PullDraftModule(ctx context.Context, name string, tag string) (*v1.Module, []*v1.ProtoFile, error)
	GetModuleTagId(ctx context.Context, moduleName string, tag string) (string, error)
	DeleteModuleTag(ctx context.Context, name string, tag string) error
	GetModuleDependencies(ctx context.Context, name string, tag string) ([]*v1.Dependency, error)
	GetTransitiveDependencies(ctx context.Context, name string, tag string) ([]*v1.Dependency, error)
	DeleteObsoleteDraftTags(ctx context.Context) error
}

type registryRepo struct {
	pool   *pgxpool.Pool
	logger *log.Helper
}

func NewRegistryRepository(pool *pgxpool.Pool, logger log.Logger) RegistryRepository {
	return &registryRepo{
		pool:   pool,
		logger: log.NewHelper(log.With(logger, "module", "data/RegistryRepository")),
	}
}

func (r *registryRepo) RegisterModule(ctx context.Context, moduleName string) error {
	// Insert module
	_, err := r.pool.Exec(ctx,
		"INSERT INTO modules (name) VALUES ($1) ON CONFLICT (name) DO NOTHING",
		moduleName)
	if err != nil {
		return fmt.Errorf("could not insert module into database: %w", err)
	}

	return nil
}

func (r *registryRepo) GetModule(ctx context.Context, name string) (*v1.Module, error) {
	return r.GetModuleWithDrafts(ctx, name, true)
}

func (r *registryRepo) GetModuleWithDrafts(ctx context.Context, name string, includeDraftTags bool) (*v1.Module, error) {
	var module v1.Module
	err := r.pool.QueryRow(ctx,
		"SELECT id, name FROM modules WHERE name = $1",
		name).Scan(&module.Id, &module.Name)
	if err != nil {
		// module not found
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("could not select module from database: %w", err)
	}

	// fetch tags
	tags, err := r.pool.Query(ctx,
		"SELECT tag FROM tags WHERE module_id = $1 ORDER BY updated_at DESC, tag DESC LIMIT $2",
		module.Id, maxModuleTags)
	if err != nil {
		// tags not found
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Infof("no tags found for module %s", name)
			return &module, nil
		}

		return nil, fmt.Errorf("could not select tags from database: %w", err)
	}
	defer tags.Close()

	for tags.Next() {
		var tag string
		err := tags.Scan(&tag)
		if err != nil {
			return nil, fmt.Errorf("could not scan tag: %w", err)
		}
		module.Tags = append(module.Tags, tag)
	}
	if err := tags.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate tags: %w", err)
	}

	if !includeDraftTags {
		return &module, nil
	}

	// fetch draft tags
	draftTags, err := r.pool.Query(ctx,
		"SELECT tag FROM draft_tags WHERE module_id = $1 ORDER BY updated_at DESC, tag DESC LIMIT $2",
		module.Id, maxModuleTags)
	if err != nil {
		// tags not found
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Infof("no draft tags found for module %s", name)
			return &module, nil
		}

		return nil, fmt.Errorf("could not select draft tags from database: %w", err)
	}
	defer draftTags.Close()

	for draftTags.Next() {
		var tag string
		err := draftTags.Scan(&tag)
		if err != nil {
			return nil, fmt.Errorf("could not scan draft tag: %w", err)
		}
		module.DraftTags = append(module.DraftTags, tag)
	}
	if err := draftTags.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate draft tags: %w", err)
	}

	return &module, nil
}

// ListModules returns a list of modules with paging support
// Token is the base64 encoded module name
func (r *registryRepo) ListModules(ctx context.Context, pageSize int, token string) ([]*v1.Module, string, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 1000 {
		pageSize = 1000
	}

	var (
		modules      []*v1.Module
		rows         pgx.Rows
		err          error
		scopedUserID = scopedUserIDFromContext(ctx)
	)

	var startAfter string
	if token != "" {
		decoded, decErr := base64.StdEncoding.DecodeString(token)
		if decErr != nil || !utf8.Valid(decoded) || bytes.IndexByte(decoded, 0) >= 0 {
			return nil, "", ErrInvalidPageToken
		}
		startAfter = string(decoded)
	}

	if scopedUserID != nil {
		rows, err = r.pool.Query(ctx, `
			SELECT id, name FROM modules
			WHERE name >= $1
				AND EXISTS (
					SELECT 1 FROM acl a
					WHERE a.user_id = $2
						AND a.permission IN ('read', 'write', 'admin')
						AND (a.module_name = '*' OR a.module_name = modules.name)
				)
			ORDER BY name ASC LIMIT $3`,
			startAfter, *scopedUserID, pageSize+1)
	} else {
		rows, err = r.pool.Query(ctx,
			"SELECT id, name FROM modules WHERE name >= $1 ORDER BY name ASC LIMIT $2",
			startAfter, pageSize+1)
	}
	if err != nil {
		return nil, "", fmt.Errorf("could not select modules from database: %w", err)
	}
	defer rows.Close()

	var rowsCount int
	var nextPageToken string

	for rows.Next() {
		module := &v1.Module{}
		err := rows.Scan(&module.Id, &module.Name)
		if err != nil {
			return nil, "", fmt.Errorf("could not scan module: %w", err)
		}

		if rowsCount < pageSize {
			modules = append(modules, module)
		} else {
			nextPageToken = base64.StdEncoding.EncodeToString([]byte(module.Name))
		}

		rowsCount++
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("could not iterate modules: %w", err)
	}
	rows.Close()

	if err := r.populateModulesDetails(ctx, modules); err != nil {
		return nil, "", err
	}

	return modules, nextPageToken, nil
}

func (r *registryRepo) populateModulesDetails(ctx context.Context, modules []*v1.Module) error {
	if len(modules) == 0 {
		return nil
	}

	moduleIDs := make([]string, len(modules))
	modulesByID := make(map[string]*v1.Module, len(modules))
	for i, mod := range modules {
		moduleIDs[i] = mod.Id
		modulesByID[mod.Id] = mod
	}

	// shortcut: sorts per-module tags in memory via existing (module_id, tag) indexes, add (module_id, updated_at DESC, tag DESC) index if modules exceed thousands of tags
	detailRows, err := r.pool.Query(ctx, `
		SELECT
			u.module_id::text,
			COALESCE((
				SELECT array_agg(t.tag ORDER BY t.updated_at DESC, t.tag DESC)
				FROM (
					SELECT tag, updated_at
					FROM tags
					WHERE module_id = u.module_id
					ORDER BY updated_at DESC, tag DESC
					LIMIT 10
				) t
			), '{}'::text[]),
			COALESCE((
				SELECT array_agg(dt.tag ORDER BY dt.updated_at DESC, dt.tag DESC)
				FROM (
					SELECT tag, updated_at
					FROM draft_tags
					WHERE module_id = u.module_id
					ORDER BY updated_at DESC, tag DESC
					LIMIT 10
				) dt
			), '{}'::text[]),
			(
				SELECT tm.meta
				FROM (
					SELECT id
					FROM tags
					WHERE module_id = u.module_id
					ORDER BY updated_at DESC, tag DESC
					LIMIT 1
				) lt
				LEFT JOIN tag_meta tm ON tm.tag_id = lt.id
			)
		FROM unnest($1::uuid[]) AS u(module_id)`, moduleIDs)
	if err != nil {
		return fmt.Errorf("could not select module details from database: %w", err)
	}
	defer detailRows.Close()

	for detailRows.Next() {
		var (
			modID     string
			tags      []string
			draftTags []string
			rawMeta   []byte
		)
		if err := detailRows.Scan(&modID, &tags, &draftTags, &rawMeta); err != nil {
			return fmt.Errorf("could not scan module details: %w", err)
		}
		mod := modulesByID[modID]
		if mod == nil {
			continue
		}
		if len(tags) > 0 {
			mod.Tags = tags
		}
		if len(draftTags) > 0 {
			mod.DraftTags = draftTags
		}
		if len(rawMeta) > 0 {
			var meta model.TagMeta
			if err := json.Unmarshal(rawMeta, &meta); err != nil {
				r.logger.Warnf("skipping corrupt tag meta for module %s: %v", mod.Name, err)
			} else if len(meta.Packages) > 0 {
				mod.Packages = meta.Packages
			}
		}
	}
	if err := detailRows.Err(); err != nil {
		return fmt.Errorf("could not iterate module details: %w", err)
	}

	return nil
}

func (r *registryRepo) DeleteModule(ctx context.Context, name string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("could not begin transaction: %w", err)
	}
	defer func(tx pgx.Tx, ctx context.Context) {
		err := tx.Rollback(ctx)
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			r.logger.Errorf("could not rollback transaction: %w", err)
		}
	}(tx, context.Background())

	// delete all protofiles
	res, err := tx.Exec(ctx,
		"DELETE FROM protofiles WHERE tag_id IN (SELECT id FROM tags WHERE module_id = (SELECT id FROM modules WHERE name = $1))",
		name)
	if err != nil {
		return fmt.Errorf("could not delete protofiles from database: %w", err)
	}

	if res.RowsAffected() > 0 {
		r.logger.Infof("deleted %d protofiles for module %s", res.RowsAffected(), name)
	}

	// delete all module dependencies (both outgoing and incoming references)
	res, err = tx.Exec(ctx,
		`DELETE FROM dependencies
		 WHERE tag_id IN (SELECT id FROM tags WHERE module_id = (SELECT id FROM modules WHERE name = $1))
		    OR dependency_tag_id IN (SELECT id FROM tags WHERE module_id = (SELECT id FROM modules WHERE name = $1))`,
		name)
	if err != nil {
		return fmt.Errorf("could not delete module dependencies from database: %w", err)
	}

	if res.RowsAffected() > 0 {
		r.logger.Infof("deleted %d dependencies for module %s", res.RowsAffected(), name)
	}

	if _, err = tx.Exec(ctx,
		"DELETE FROM proto_parsed_data WHERE tag_id IN (SELECT id FROM tags WHERE module_id = (SELECT id FROM modules WHERE name = $1))",
		name); err != nil {
		return fmt.Errorf("could not delete parsed proto data from database: %w", err)
	}
	if _, err = tx.Exec(ctx,
		"DELETE FROM tag_meta WHERE tag_id IN (SELECT id FROM tags WHERE module_id = (SELECT id FROM modules WHERE name = $1))",
		name); err != nil {
		return fmt.Errorf("could not delete tag metadata from database: %w", err)
	}

	// delete all module tags
	res, err = tx.Exec(ctx,
		"DELETE FROM tags WHERE module_id = (SELECT id FROM modules WHERE name = $1)",
		name)
	if err != nil {
		return fmt.Errorf("could not delete module tags from database: %w", err)
	}

	if res.RowsAffected() > 0 {
		r.logger.Infof("deleted %d tags for module %s", res.RowsAffected(), name)
	}

	// delete all module draft tags
	res, err = tx.Exec(ctx,
		"DELETE FROM draft_tags WHERE module_id = (SELECT id FROM modules WHERE name = $1)",
		name)
	if err != nil {
		return fmt.Errorf("could not delete module draft tags from database: %w", err)
	}

	if res.RowsAffected() > 0 {
		r.logger.Infof("deleted %d draft tags for module %s", res.RowsAffected(), name)
	}

	// delete module
	res, err = tx.Exec(ctx,
		"DELETE FROM modules WHERE name = $1",
		name)
	if err != nil {
		return fmt.Errorf("could not delete module from database: %w", err)
	}

	if res.RowsAffected() == 0 {
		return ErrModuleNotFound
	}

	if _, err = tx.Exec(ctx, "DELETE FROM acl WHERE module_name = $1 AND module_name <> '*'", name); err != nil {
		return fmt.Errorf("could not delete module acl entries from database: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("could not commit module deletion: %w", err)
	}

	r.logger.Infof("deleted module %s", name)
	return nil
}

func (r *registryRepo) PushModule(ctx context.Context, name string, tag string, protofiles []*v1.ProtoFile, dependencies ...*v1.Dependency) (*v1.Module, error) {
	// check if module exists
	module, err := r.GetModuleWithDrafts(ctx, name, false)
	if err != nil {
		return nil, fmt.Errorf("could not get module: %w", err)
	}

	if module == nil {
		return nil, ErrModuleNotFound
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not begin transaction: %w", err)
	}
	defer func(tx pgx.Tx, ctx context.Context) {
		err := tx.Rollback(ctx)
		if err != nil {
			if !errors.Is(err, pgx.ErrTxClosed) {
				r.logger.Errorf("could not rollback transaction: %w", err)
			}
		}
	}(tx, context.Background())

	var tagId string
	err = tx.QueryRow(ctx,
		"INSERT INTO tags (module_id, tag) VALUES ($1, $2) ON CONFLICT (module_id, tag) DO NOTHING RETURNING id",
		module.Id, tag).Scan(&tagId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTagAlreadyExists
		}
		return nil, fmt.Errorf("could not insert tag into database: %w", err)
	}

	// insert protofiles
	for _, protofile := range protofiles {
		_, err = tx.Exec(ctx,
			"INSERT INTO protofiles (tag_id, filename, content, content_hash) VALUES ($1, $2, $3, $4)",
			tagId, protofile.Filename, protofile.Content, computeHash(protofile.Content))
		if err != nil {
			return nil, fmt.Errorf("could not insert protofile into database: %w", err)
		}
	}

	// insert dependencies atomically within the same transaction
	if err := r.insertDependenciesTx(ctx, tx, tagId, dependencies); err != nil {
		return nil, err
	}

	err = tx.Commit(ctx)
	if err != nil {
		r.logger.Errorf("could not commit transaction: %w", err)
		return nil, fmt.Errorf("could not push the module. internal error")
	}

	module.Tags = append([]string{tag}, module.Tags...)
	if len(module.Tags) > maxModuleTags {
		module.Tags = module.Tags[:maxModuleTags]
	}

	return module, nil
}

func (r *registryRepo) PullModule(ctx context.Context, name string, tag string) (*v1.Module, []*v1.ProtoFile, error) {
	// check if module exists
	module, err := r.GetModuleWithDrafts(ctx, name, false)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get module: %w", err)
	}

	if module == nil {
		return nil, nil, ErrModuleNotFound
	}

	tagId, err := r.GetModuleTagId(ctx, name, tag)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get tag id: %w", err)
	}

	if tagId == "" {
		return nil, nil, ErrTagNotFound
	}

	// fetch protofiles
	protofilesRows, err := r.pool.Query(ctx,
		"SELECT filename, content FROM protofiles WHERE tag_id = $1",
		tagId)
	if err != nil {
		return nil, nil, fmt.Errorf("could not select protofiles from database: %w", err)
	}
	defer protofilesRows.Close()

	var protofiles []*v1.ProtoFile

	for protofilesRows.Next() {
		protofile := &v1.ProtoFile{}
		err := protofilesRows.Scan(&protofile.Filename, &protofile.Content)
		if err != nil {
			return nil, nil, fmt.Errorf("could not scan protofile: %w", err)
		}

		protofiles = append(protofiles, protofile)
	}
	if err := protofilesRows.Err(); err != nil {
		return nil, nil, fmt.Errorf("could not iterate protofiles: %w", err)
	}

	return module, protofiles, nil
}

func (r *registryRepo) PullDraftModule(ctx context.Context, name string, tag string) (*v1.Module, []*v1.ProtoFile, error) {
	// fetch module with GetModule method and get all other from draft_tags table
	module, err := r.GetModule(ctx, name)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get module: %w", err)
	}

	if module == nil {
		return nil, nil, ErrModuleNotFound
	}

	// get info from draft_tags table
	var protofilesJson string
	err = r.pool.QueryRow(ctx,
		"SELECT proto_files FROM draft_tags WHERE module_id = $1 AND tag = $2",
		module.Id, tag).Scan(&protofilesJson)
	if err != nil {
		// if not found raise specific error
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrTagNotFound
		}
		return nil, nil, fmt.Errorf("could not select draft tag from database: %w", err)
	}

	// unmarshal protofiles
	var protofiles []*v1.ProtoFile
	err = json.Unmarshal([]byte(protofilesJson), &protofiles)
	if err != nil {
		return nil, nil, fmt.Errorf("could not unmarshal protofiles: %w", err)
	}

	return module, protofiles, nil
}

func (r *registryRepo) DeleteModuleTag(ctx context.Context, name string, tag string) error {
	tagId, err := r.GetModuleTagId(ctx, name, tag)
	if err != nil {
		return fmt.Errorf("could not get tag id: %w", err)
	}

	if tagId == "" {
		res, err := r.pool.Exec(ctx,
			"DELETE FROM draft_tags WHERE module_id = (SELECT id FROM modules WHERE name = $1) AND tag = $2",
			name, tag)
		if err != nil {
			return fmt.Errorf("could not delete draft tag from database: %w", err)
		}
		if res.RowsAffected() == 0 {
			return ErrTagNotFound
		}
		r.logger.Infof("deleted draft tag %s", tag)
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("could not begin transaction: %w", err)
	}
	defer func(tx pgx.Tx, ctx context.Context) {
		err := tx.Rollback(ctx)
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			r.logger.Errorf("could not rollback transaction: %w", err)
		}
	}(tx, context.Background())

	// delete protofiles
	res, err := tx.Exec(ctx,
		"DELETE FROM protofiles WHERE tag_id = $1",
		tagId)
	if err != nil {
		return fmt.Errorf("could not delete protofiles from database: %w", err)
	}

	if res.RowsAffected() > 0 {
		r.logger.Infof("deleted %d protofiles for tag %s", res.RowsAffected(), tag)
	}

	// delete dependencies
	res, err = tx.Exec(ctx,
		"DELETE FROM dependencies WHERE tag_id = $1 OR dependency_tag_id = $1",
		tagId)
	if err != nil {
		return fmt.Errorf("could not delete dependencies from database: %w", err)
	}

	if res.RowsAffected() > 0 {
		r.logger.Infof("deleted %d dependencies for tag %s", res.RowsAffected(), tag)
	}

	if _, err = tx.Exec(ctx, "DELETE FROM proto_parsed_data WHERE tag_id = $1", tagId); err != nil {
		return fmt.Errorf("could not delete parsed proto data from database: %w", err)
	}
	if _, err = tx.Exec(ctx, "DELETE FROM tag_meta WHERE tag_id = $1", tagId); err != nil {
		return fmt.Errorf("could not delete tag metadata from database: %w", err)
	}

	// delete tag
	res, err = tx.Exec(ctx,
		"DELETE FROM tags WHERE id = $1",
		tagId)
	if err != nil {
		return fmt.Errorf("could not delete tag from database: %w", err)
	}

	if res.RowsAffected() == 0 {
		return ErrTagNotFound
	}

	if _, err = tx.Exec(ctx,
		"DELETE FROM draft_tags WHERE module_id = (SELECT id FROM modules WHERE name = $1) AND tag = $2",
		name, tag); err != nil {
		return fmt.Errorf("could not delete draft tag from database: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("could not commit tag deletion: %w", err)
	}

	r.logger.Infof("deleted tag %s", tag)
	return nil
}

func (r *registryRepo) insertDependenciesTx(ctx context.Context, tx pgx.Tx, tagId string, dependencies []*v1.Dependency) error {
	for _, dependency := range dependencies {
		if dependency == nil {
			continue
		}
		var dependencyTagId string
		err := tx.QueryRow(ctx,
			"SELECT id FROM tags WHERE module_id = (SELECT id FROM modules WHERE name = $1) AND tag = $2",
			dependency.Name, dependency.Tag).Scan(&dependencyTagId)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("dependency %s:%s not found: %w", dependency.Name, dependency.Tag, ErrDependencyTagNotFound)
			}
			return fmt.Errorf("could not find tag %s for module %s: %w", dependency.Tag, dependency.Name, err)
		}

		_, err = tx.Exec(ctx,
			"INSERT INTO dependencies (tag_id, dependency_tag_id) VALUES ($1, $2) ON CONFLICT (tag_id, dependency_tag_id) DO NOTHING",
			tagId, dependencyTagId)
		if err != nil {
			return fmt.Errorf("could not insert dependency into database: %w", err)
		}
	}

	return nil
}

func (r *registryRepo) GetModuleDependencies(ctx context.Context, name string, tag string) ([]*v1.Dependency, error) {
	var dependencies []*v1.Dependency
	// find the latest tag if tag is empty
	if tag == "" {
		err := r.pool.QueryRow(ctx,
			"SELECT tag FROM tags WHERE module_id = (SELECT id FROM modules WHERE name = $1) ORDER BY updated_at DESC, tag DESC LIMIT 1",
			name).Scan(&tag)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return dependencies, nil
			}
			return nil, fmt.Errorf("could not select tag from database: %w", err)
		}
	}

	tagId, err := r.GetModuleTagId(ctx, name, tag)
	if err != nil {
		return nil, fmt.Errorf("could not get tag id: %w", err)
	}

	if tagId == "" {
		return nil, ErrTagNotFound
	}

	// Fetch all dependencies in a single JOIN query to avoid N+1 queries and connection pool contention
	rows, err := r.pool.Query(ctx, `
		SELECT m.name, t.tag
		FROM dependencies d
		JOIN tags t ON d.dependency_tag_id = t.id
		JOIN modules m ON t.module_id = m.id
		WHERE d.tag_id = $1
			AND ($2::uuid IS NULL OR EXISTS (
				SELECT 1 FROM acl a
				WHERE a.user_id = $2
					AND a.permission IN ('read', 'write', 'admin')
					AND (a.module_name = '*' OR a.module_name = m.name)
			))
		ORDER BY m.name ASC, t.tag ASC
	`, tagId, scopedUserIDFromContext(ctx))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []*v1.Dependency{}, nil
		}
		return nil, fmt.Errorf("could not select dependencies from database: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var dependencyName, dependencyTag string
		if err := rows.Scan(&dependencyName, &dependencyTag); err != nil {
			return nil, fmt.Errorf("could not scan dependency: %w", err)
		}

		dependencies = append(dependencies, &v1.Dependency{
			Name: dependencyName,
			Tag:  dependencyTag,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate dependencies: %w", err)
	}

	return dependencies, nil
}

func (r *registryRepo) PushDraftModule(ctx context.Context, name string, tag string, protofiles []*v1.ProtoFile, dependencies []*v1.Dependency) (*v1.Module, error) {
	// check if module exists
	module, err := r.GetModule(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("could not get module: %w", err)
	}

	if module == nil {
		return nil, ErrModuleNotFound
	}

	// serialize protofiles and dependencies in json
	protofilesJson, err := json.Marshal(protofiles)
	if err != nil {
		return nil, fmt.Errorf("could not serialize protofiles: %w", err)
	}

	dependenciesJson, err := json.Marshal(dependencies)
	if err != nil {
		return nil, fmt.Errorf("could not serialize dependencies: %w", err)
	}

	// create the tag
	// if exists update proto_files, dependencies and updated_at
	_, err = r.pool.Exec(ctx, "INSERT INTO draft_tags (module_id, tag, proto_files, dependencies) VALUES ($1, $2, $3, $4) ON CONFLICT (module_id, tag) DO UPDATE SET proto_files = $3, dependencies = $4, updated_at = NOW()",
		module.Id, tag, protofilesJson, dependenciesJson)
	if err != nil {
		return nil, fmt.Errorf("could not insert draft tag into database: %w", err)
	}

	// append if not exists
	// check by DraftTags property
	var draftTagExists bool
	for _, t := range module.DraftTags {
		if t == tag {
			draftTagExists = true
			break
		}
	}

	if !draftTagExists {
		module.DraftTags = append(module.DraftTags, tag)
	}

	return module, nil
}

func (r *registryRepo) GetModuleTagId(ctx context.Context, moduleName string, tag string) (string, error) {
	// check if tag exists
	var tagId string
	err := r.pool.QueryRow(ctx,
		"SELECT id FROM tags WHERE module_id = (SELECT id FROM modules WHERE name = $1) AND tag = $2",
		moduleName, tag).Scan(&tagId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("could not select tag from database: %w", err)
	}

	return tagId, nil
}

// depKey is a struct used as a map key to track visited dependencies,
// avoiding string concatenation collision risks.
type depKey struct {
	name string
	tag  string
}

func (r *registryRepo) GetTransitiveDependencies(ctx context.Context, name string, tag string) ([]*v1.Dependency, error) {
	// Get direct dependencies first
	directDeps, err := r.GetModuleDependencies(ctx, name, tag)
	if err != nil {
		return nil, fmt.Errorf("could not get direct dependencies: %w", err)
	}

	// Mark all direct dependencies
	result := make([]*v1.Dependency, 0, len(directDeps))
	for _, dep := range directDeps {
		result = append(result, &v1.Dependency{
			Name:           dep.Name,
			Tag:            dep.Tag,
			DependencyType: "direct",
		})
	}

	// Track visited modules to avoid cycles and duplicates
	visited := make(map[depKey]bool)
	visited[depKey{name: name, tag: tag}] = true
	for _, dep := range directDeps {
		visited[depKey{name: dep.Name, tag: dep.Tag}] = true
	}

	// BFS to resolve transitive dependencies
	queue := make([]*v1.Dependency, len(directDeps))
	copy(queue, directDeps)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Get dependencies of the current dependency
		transitiveDeps, err := r.GetModuleDependencies(ctx, current.Name, current.Tag)
		if err != nil {
			r.logger.Warnf("could not get dependencies for %s:%s: %v", current.Name, current.Tag, err)
			continue
		}

		for _, dep := range transitiveDeps {
			key := depKey{name: dep.Name, tag: dep.Tag}
			if !visited[key] {
				visited[key] = true
				result = append(result, &v1.Dependency{
					Name:           dep.Name,
					Tag:            dep.Tag,
					DependencyType: "transitive",
				})
				queue = append(queue, dep)
			}
		}
	}

	return result, nil
}

// DeleteObsoleteDraftTags deletes all draft tags
// that are older than 7 days
func (r *registryRepo) DeleteObsoleteDraftTags(ctx context.Context) error {
	res, err := r.pool.Exec(ctx,
		"DELETE FROM draft_tags WHERE updated_at < NOW() - INTERVAL '7 days'")
	if err != nil {
		return fmt.Errorf("could not delete draft tags from database: %w", err)
	}

	if res.RowsAffected() > 0 {
		r.logger.Infof("deleted %d draft tags", res.RowsAffected())
	}

	return nil
}
