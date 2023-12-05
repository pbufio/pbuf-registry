package data

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
)

var TagNotFoundError = errors.New("tag not found")

type RegistryRepository interface {
	RegisterModule(ctx context.Context, moduleName string) error
	GetModule(ctx context.Context, name string) (*v1.Module, error)
	ListModules(ctx context.Context, pageSize int, token string) ([]*v1.Module, string, error)
	DeleteModule(ctx context.Context, name string) error
	PushModule(ctx context.Context, name string, tag string, protofiles []*v1.ProtoFile) (*v1.Module, error)
	PushDraftModule(ctx context.Context, name string, tag string, protofiles []*v1.ProtoFile, dependencies []*v1.Dependency) (*v1.Module, error)
	PullModule(ctx context.Context, name string, tag string) (*v1.Module, []*v1.ProtoFile, error)
	PullDraftModule(ctx context.Context, name string, tag string) (*v1.Module, []*v1.ProtoFile, error)
	GetModuleTagId(ctx context.Context, moduleName string, tag string) (string, error)
	DeleteModuleTag(ctx context.Context, name string, tag string) error
	AddModuleDependencies(ctx context.Context, name string, tag string, dependencies []*v1.Dependency) error
	GetModuleDependencies(ctx context.Context, name string, tag string) ([]*v1.Dependency, error)
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
		"SELECT tag FROM tags WHERE module_id = $1 ORDER BY updated_at DESC LIMIT 10",
		module.Id)
	if err != nil {
		// tags not found
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Infof("no tags found for module %s", name)
			return &module, nil
		}

		return nil, fmt.Errorf("could not select tags from database: %w", err)
	}

	for tags.Next() {
		var tag string
		err := tags.Scan(&tag)
		if err != nil {
			return nil, fmt.Errorf("could not scan tag: %w", err)
		}
		module.Tags = append(module.Tags, tag)
	}

	// fetch draft tags
	draftTags, err := r.pool.Query(ctx,
		"SELECT tag FROM draft_tags WHERE module_id = $1 ORDER BY updated_at DESC LIMIT 10",
		module.Id)
	if err != nil {
		// tags not found
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Infof("no draft tags found for module %s", name)
			return &module, nil
		}

		return nil, fmt.Errorf("could not select draft tags from database: %w", err)
	}

	for draftTags.Next() {
		var tag string
		err := draftTags.Scan(&tag)
		if err != nil {
			return nil, fmt.Errorf("could not scan draft tag: %w", err)
		}
		module.DraftTags = append(module.DraftTags, tag)
	}

	return &module, nil
}

// ListModules returns a list of modules with paging support
// Token is the base64 encoded module name
func (r *registryRepo) ListModules(ctx context.Context, pageSize int, token string) ([]*v1.Module, string, error) {
	var modules []*v1.Module

	query := "SELECT id, name FROM modules"
	if token != "" {
		decoded, err := base64.StdEncoding.DecodeString(token)
		if err != nil {
			return nil, "", fmt.Errorf("could not decode token: %w", err)
		}
		query += fmt.Sprintf(" WHERE name >= '%s'", decoded)
	}

	query += fmt.Sprintf(" ORDER BY name ASC LIMIT %d", pageSize+1)

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, "", fmt.Errorf("could not select modules from database: %w", err)
	}

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

	return modules, nextPageToken, nil
}

func (r *registryRepo) DeleteModule(ctx context.Context, name string) error {
	// delete all protofiles
	res, err := r.pool.Exec(ctx,
		"DELETE FROM protofiles WHERE tag_id IN (SELECT id FROM tags WHERE module_id = (SELECT id FROM modules WHERE name = $1))",
		name)
	if err != nil {
		return fmt.Errorf("could not delete protofiles from database: %w", err)
	}

	if res.RowsAffected() > 0 {
		r.logger.Infof("deleted %d protofiles for module %s", res.RowsAffected(), name)
	}

	// delete all module dependencies
	res, err = r.pool.Exec(ctx,
		"DELETE FROM dependencies WHERE tag_id IN (SELECT id FROM tags WHERE module_id = (SELECT id FROM modules WHERE name = $1))",
		name)
	if err != nil {
		return fmt.Errorf("could not delete module dependencies from database: %w", err)
	}

	if res.RowsAffected() > 0 {
		r.logger.Infof("deleted %d dependencies for module %s", res.RowsAffected(), name)
	}

	// delete all module tags
	res, err = r.pool.Exec(ctx,
		"DELETE FROM tags WHERE module_id = (SELECT id FROM modules WHERE name = $1)",
		name)
	if err != nil {
		return fmt.Errorf("could not delete module tags from database: %w", err)
	}

	if res.RowsAffected() > 0 {
		r.logger.Infof("deleted %d tags for module %s", res.RowsAffected(), name)
	}

	// delete all module draft tags
	res, err = r.pool.Exec(ctx,
		"DELETE FROM draft_tags WHERE module_id = (SELECT id FROM modules WHERE name = $1)",
		name)
	if err != nil {
		return fmt.Errorf("could not delete module draft tags from database: %w", err)
	}

	if res.RowsAffected() > 0 {
		r.logger.Infof("deleted %d draft tags for module %s", res.RowsAffected(), name)
	}

	// delete module
	res, err = r.pool.Exec(ctx,
		"DELETE FROM modules WHERE name = $1",
		name)
	if err != nil {
		return fmt.Errorf("could not delete module from database: %w", err)
	}

	if res.RowsAffected() > 0 {
		r.logger.Infof("deleted module %s", name)
	} else {
		return errors.New("module not found")
	}

	return nil
}

func (r *registryRepo) PushModule(ctx context.Context, name string, tag string, protofiles []*v1.ProtoFile) (*v1.Module, error) {
	// check if module exists
	module, err := r.GetModule(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("could not get module: %w", err)
	}

	if module == nil {
		return nil, errors.New("module not found")
	}

	// check if tag exists
	for _, t := range module.Tags {
		if t == tag {
			return nil, errors.New("tag already exists")
		}
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

	// create the tag
	_, err = tx.Exec(ctx, "INSERT INTO tags (module_id, tag) VALUES ($1, $2)", module.Id, tag)
	if err != nil {
		return nil, fmt.Errorf("could not insert tag into database: %w", err)
	}

	var tagId string

	// fetch the new tag
	err = tx.QueryRow(ctx,
		"SELECT id FROM tags WHERE module_id = $1 AND tag = $2",
		module.Id, tag).Scan(&tagId)
	if err != nil {
		return nil, fmt.Errorf("could not select tag from database: %w", err)
	}

	// insert protofiles
	for _, protofile := range protofiles {
		_, err = tx.Exec(ctx,
			"INSERT INTO protofiles (tag_id, filename, content) VALUES ($1, $2, $3)",
			tagId, protofile.Filename, protofile.Content)
		if err != nil {
			return nil, fmt.Errorf("could not insert protofile into database: %w", err)
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		r.logger.Errorf("could not commit transaction: %w", err)
		return nil, fmt.Errorf("could not push the module. internal error")
	}

	module.Tags = append(module.Tags, tag)

	return module, nil
}

func (r *registryRepo) PullModule(ctx context.Context, name string, tag string) (*v1.Module, []*v1.ProtoFile, error) {
	// check if module exists
	module, err := r.GetModule(ctx, name)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get module: %w", err)
	}

	if module == nil {
		return nil, nil, errors.New("module not found")
	}

	tagId, err := r.GetModuleTagId(ctx, name, tag)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get tag id: %w", err)
	}

	if tagId == "" {
		return nil, nil, TagNotFoundError
	}

	// fetch protofiles
	protofilesRows, err := r.pool.Query(ctx,
		"SELECT filename, content FROM protofiles WHERE tag_id = $1",
		tagId)
	if err != nil {
		return nil, nil, fmt.Errorf("could not select protofiles from database: %w", err)
	}

	var protofiles []*v1.ProtoFile

	for protofilesRows.Next() {
		protofile := &v1.ProtoFile{}
		err := protofilesRows.Scan(&protofile.Filename, &protofile.Content)
		if err != nil {
			return nil, nil, fmt.Errorf("could not scan protofile: %w", err)
		}

		protofiles = append(protofiles, protofile)
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
		return nil, nil, errors.New("module not found")
	}

	// get info from draft_tags table
	var protofilesJson string
	err = r.pool.QueryRow(ctx,
		"SELECT proto_files FROM draft_tags WHERE module_id = $1 AND tag = $2",
		module.Id, tag).Scan(&protofilesJson)
	if err != nil {
		// if not found raise specific error
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, errors.New("draft tag not found")
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
		return TagNotFoundError
	}

	// delete protofiles
	res, err := r.pool.Exec(ctx,
		"DELETE FROM protofiles WHERE tag_id = $1",
		tagId)
	if err != nil {
		return fmt.Errorf("could not delete protofiles from database: %w", err)
	}

	if res.RowsAffected() > 0 {
		r.logger.Infof("deleted %d protofiles for tag %s", res.RowsAffected(), tag)
	}

	// delete dependencies
	res, err = r.pool.Exec(ctx,
		"DELETE FROM dependencies WHERE tag_id = $1 OR dependency_tag_id = $1",
		tagId)
	if err != nil {
		return fmt.Errorf("could not delete dependencies from database: %w", err)
	}

	if res.RowsAffected() > 0 {
		r.logger.Infof("deleted %d dependencies for tag %s", res.RowsAffected(), tag)
	}

	// delete tag
	res, err = r.pool.Exec(ctx,
		"DELETE FROM tags WHERE id = $1",
		tagId)
	if err != nil {
		return fmt.Errorf("could not delete tag from database: %w", err)
	}

	if res.RowsAffected() > 0 {
		r.logger.Infof("deleted tag %s", tag)
	} else {
		return TagNotFoundError
	}

	return nil
}

func (r *registryRepo) AddModuleDependencies(ctx context.Context, name string, tag string, dependencies []*v1.Dependency) error {
	// find the tag id by name and tag
	tagId, err := r.GetModuleTagId(ctx, name, tag)
	if err != nil {
		return fmt.Errorf("could not get tag id: %w", err)
	}

	if tagId == "" {
		return TagNotFoundError
	}

	// for each dependency find tag id and insert into dependencies table
	for _, dependency := range dependencies {
		var dependencyTagId string
		err := r.pool.QueryRow(ctx,
			"SELECT id FROM tags WHERE module_id = (SELECT id FROM modules WHERE name = $1) AND tag = $2",
			dependency.Name, dependency.Tag).Scan(&dependencyTagId)
		if err != nil {
			return fmt.Errorf("could not find tag %s for module %s: %w", dependency.Tag, dependency.Name, err)
		}

		_, err = r.pool.Exec(ctx,
			"INSERT INTO dependencies (tag_id, dependency_tag_id) VALUES ($1, $2)",
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
			"SELECT tag FROM tags WHERE module_id = (SELECT id FROM modules WHERE name = $1) ORDER BY updated_at DESC LIMIT 1",
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
		return nil, TagNotFoundError
	}

	// find all dependencies
	rows, err := r.pool.Query(ctx,
		"SELECT dependency_tag_id FROM dependencies WHERE tag_id = $1",
		tagId)
	if err != nil {
		// if no dependencies found, return empty slice
		if errors.Is(err, pgx.ErrNoRows) {
			return []*v1.Dependency{}, nil
		}
		return nil, fmt.Errorf("could not select dependencies from database: %w", err)
	}

	for rows.Next() {
		var dependencyTagId string
		err := rows.Scan(&dependencyTagId)
		if err != nil {
			return nil, fmt.Errorf("could not scan dependency: %w", err)
		}

		// find the module name and tag by dependency tag id
		var dependencyName string
		var dependencyTag string
		err = r.pool.QueryRow(ctx,
			"SELECT modules.name, tags.tag FROM modules JOIN tags ON modules.id = tags.module_id WHERE tags.id = $1",
			dependencyTagId).Scan(&dependencyName, &dependencyTag)
		if err != nil {
			return nil, fmt.Errorf("could not find module and tag for dependency: %w", err)
		}

		dependencies = append(dependencies, &v1.Dependency{
			Name: dependencyName,
			Tag:  dependencyTag,
		})
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
		return nil, errors.New("module not found")
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
