package data

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/pbufio/pbuf-registry/internal/utils"
	"github.com/yoheimuta/go-protoparser/v4/interpret/unordered"
)

type MetadataRepository interface {
	GetUnprocessedTagIds(ctx context.Context) ([]string, error)
	GetProtoFilesForTagId(ctx context.Context, tagId string) ([]*v1.ProtoFile, error)
	SaveParsedProtoFiles(ctx context.Context, tagId string, files []*model.ParsedProtoFile) error
	GetParsedProtoFiles(ctx context.Context, tagId string) ([]*model.ParsedProtoFile, error)
	GetTagMetaByTagId(ctx context.Context, tagId string) (*model.TagMeta, error)
}

type metadataRepo struct {
	pool   *pgxpool.Pool
	logger *log.Helper
}

func (m metadataRepo) GetUnprocessedTagIds(ctx context.Context) ([]string, error) {
	var tagIds []string

	rows, err := m.pool.Query(ctx, "SELECT id FROM tags WHERE is_processed = false")
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tagIds, nil
		}
		m.logger.Errorf("error getting unprocessed tag ids: %v", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var tagId string
		err = rows.Scan(&tagId)
		if err != nil {
			m.logger.Errorf("error scanning tag id: %v", err)
			return nil, err
		}
		tagIds = append(tagIds, tagId)
	}

	return tagIds, nil
}

func (m metadataRepo) GetProtoFilesForTagId(ctx context.Context, tagId string) ([]*v1.ProtoFile, error) {
	var protoFiles []*v1.ProtoFile

	rows, err := m.pool.Query(ctx, "SELECT filename, content FROM protofiles WHERE tag_id = $1", tagId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return protoFiles, nil
		}
		m.logger.Errorf("error getting proto files for tag id %s: %v", tagId, err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var filename, content string
		err = rows.Scan(&filename, &content)
		if err != nil {
			m.logger.Errorf("error scanning proto file: %v", err)
			return nil, err
		}
		protoFiles = append(protoFiles, &v1.ProtoFile{
			Filename: filename,
			Content:  content,
		})
	}

	return protoFiles, nil
}

func (m metadataRepo) SaveParsedProtoFiles(ctx context.Context, tagId string, files []*model.ParsedProtoFile) error {
	meta, err := utils.RetrieveMeta(files)
	if err != nil {
		m.logger.Errorf("error retrieving tag meta: %v", err)
		return err
	}

	tx, err := m.pool.Begin(ctx)
	if err != nil {
		m.logger.Errorf("error starting transaction: %v", err)
		return err
	}
	defer func(tx pgx.Tx, ctx context.Context) {
		err := tx.Rollback(ctx)
		if err != nil {
			if !errors.Is(err, pgx.ErrTxClosed) {
				m.logger.Errorf("error rolling back transaction: %v", err)
			}
		}
	}(tx, ctx)

	for _, file := range files {
		// add to proto_parsed_data table
		// on duplicate update json
		_, err = tx.Exec(ctx, "INSERT INTO proto_parsed_data (tag_id, filename, json) VALUES ($1, $2, $3) ON CONFLICT (tag_id, filename) DO UPDATE SET json = $3", tagId, file.Filename, file.ProtoJson)
		if err != nil {
			m.logger.Errorf("error inserting parsed proto file: %v", err)
			return err
		}
	}

	// add to tag_meta table
	// on duplicate update json
	_, err = tx.Exec(ctx, "INSERT INTO tag_meta (tag_id, meta) VALUES ($1, $2) ON CONFLICT (tag_id) DO UPDATE SET meta = $2", tagId, meta)
	if err != nil {
		m.logger.Errorf("error inserting tag meta: %v", err)
		return err
	}

	// update tag to processed
	_, err = tx.Exec(ctx, "UPDATE tags SET is_processed = true WHERE id = $1", tagId)
	if err != nil {
		m.logger.Errorf("error updating tag to processed: %v", err)
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		m.logger.Errorf("error committing transaction: %v", err)
		return err
	}

	return nil
}

func (m *metadataRepo) GetTagMetaByTagId(ctx context.Context, tagId string) (*model.TagMeta, error) {
	var meta model.TagMeta

	err := m.pool.QueryRow(ctx, "SELECT meta FROM tag_meta WHERE tag_id = $1", tagId).Scan(&meta)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		m.logger.Errorf("error getting tag meta for tag id %s: %v", tagId, err)
		return nil, err
	}

	return &meta, nil
}

func (m metadataRepo) GetParsedProtoFiles(ctx context.Context, tagId string) ([]*model.ParsedProtoFile, error) {
	var parsedProtoFiles []*model.ParsedProtoFile

	rows, err := m.pool.Query(ctx, "SELECT filename, json FROM proto_parsed_data WHERE tag_id = $1", tagId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return parsedProtoFiles, nil
		}
		m.logger.Errorf("error getting parsed proto files for tag id %s: %v", tagId, err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var filename, protoJson string
		var proto *unordered.Proto
		err = rows.Scan(&filename, &protoJson)
		if err != nil {
			m.logger.Errorf("error scanning parsed proto file: %v", err)
			return nil, err
		}

		err = json.Unmarshal([]byte(protoJson), &proto)
		if err != nil {
			m.logger.Errorf("error unmarshaling json: %v", err)
			return nil, err
		}

		parsedProtoFiles = append(parsedProtoFiles, &model.ParsedProtoFile{
			Filename: filename,
			Proto:    proto,
		})
	}

	return parsedProtoFiles, nil
}

// NewMetadataRepository create a new metadata repository with pool
func NewMetadataRepository(pool *pgxpool.Pool, logger log.Logger) MetadataRepository {
	return &metadataRepo{
		pool:   pool,
		logger: log.NewHelper(log.With(logger, "module", "data/MetadataRepository")),
	}
}
