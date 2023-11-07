package data

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/google/martian/log"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	v1 "github.com/pbufio/pbuf-registry/gen/v1"
)

type RegistryRepository interface {
	RegisterModule(ctx context.Context, moduleName string) error
	GetModule(ctx context.Context, name string) (*v1.Module, error)
	ListModules(ctx context.Context, pageSize int, token string) ([]*v1.Module, string, error)
	DeleteModule(ctx context.Context, name string) error
	PushModule(ctx context.Context, name string, tag string, protofiles []*v1.ProtoFile) (*v1.Module, error)
	PullModule(ctx context.Context, name string, tag string) (*v1.Module, []*v1.ProtoFile, error)
}

type registryRepository struct {
	pool *pgxpool.Pool
}

func NewRegistryRepository(pool *pgxpool.Pool) RegistryRepository {
	return &registryRepository{
		pool: pool,
	}
}

func (r *registryRepository) RegisterModule(ctx context.Context, moduleName string) error {
	// Insert module
	_, err := r.pool.Exec(ctx,
		"INSERT INTO modules (name) VALUES ($1) ON CONFLICT (name) DO NOTHING",
		moduleName)
	if err != nil {
		return fmt.Errorf("could not insert module into database: %w", err)
	}

	return nil
}

func (r *registryRepository) GetModule(ctx context.Context, name string) (*v1.Module, error) {
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
		"SELECT tag FROM tags WHERE module_id = $1",
		module.Id)
	if err != nil {
		// tags not found
		if errors.Is(err, pgx.ErrNoRows) {
			log.Infof("no tags found for module %s", name)
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

	return &module, nil
}

// ListModules returns a list of modules with paging support
// Token is the base64 encoded module name
func (r *registryRepository) ListModules(ctx context.Context, pageSize int, token string) ([]*v1.Module, string, error) {
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

func (r *registryRepository) DeleteModule(ctx context.Context, name string) error {
	// delete all module tags
	res, err := r.pool.Exec(ctx,
		"DELETE FROM tags WHERE module_id = (SELECT id FROM modules WHERE name = $1)",
		name)
	if err != nil {
		return fmt.Errorf("could not delete module tags from database: %w", err)
	}

	if res.RowsAffected() > 0 {
		log.Infof("deleted %d tags for module %s", res.RowsAffected(), name)
	}

	// delete module
	res, err = r.pool.Exec(ctx,
		"DELETE FROM modules WHERE name = $1",
		name)
	if err != nil {
		return fmt.Errorf("could not delete module from database: %w", err)
	}

	if res.RowsAffected() > 0 {
		log.Infof("deleted module %s", name)
	} else {
		return errors.New("module not found")
	}

	return nil
}

func (r *registryRepository) PushModule(ctx context.Context, name string, tag string, protofiles []*v1.ProtoFile) (*v1.Module, error) {
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

	// create the tag
	_, err = r.pool.Exec(ctx, "INSERT INTO tags (module_id, tag) VALUES ($1, $2)", module.Id, tag)
	if err != nil {
		return nil, fmt.Errorf("could not insert tag into database: %w", err)
	}

	var tagId string

	// fetch the new tag
	err = r.pool.QueryRow(ctx,
		"SELECT id FROM tags WHERE module_id = $1 AND tag = $2",
		module.Id, tag).Scan(&tagId)
	if err != nil {
		return nil, fmt.Errorf("could not select tag from database: %w", err)
	}

	// insert protofiles
	for _, protofile := range protofiles {
		_, err = r.pool.Exec(ctx,
			"INSERT INTO protofiles (tag_id, filename, content) VALUES ($1, $2, $3)",
			tagId, protofile.Filename, protofile.Content)
		if err != nil {
			return nil, fmt.Errorf("could not insert protofile into database: %w", err)
		}
	}

	module.Tags = append(module.Tags, tag)

	return module, nil
}

func (r *registryRepository) PullModule(ctx context.Context, name string, tag string) (*v1.Module, []*v1.ProtoFile, error) {
	// check if module exists
	module, err := r.GetModule(ctx, name)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get module: %w", err)
	}

	if module == nil {
		return nil, nil, errors.New("module not found")
	}

	// check if tag exists
	var tagId string
	err = r.pool.QueryRow(ctx,
		"SELECT id FROM tags WHERE module_id = $1 AND tag = $2",
		module.Id, tag).Scan(&tagId)
	if err != nil {
		return nil, nil, fmt.Errorf("could not select tag from database: %w", err)
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
