package search

import "context"

// Service is the search seam used by modules: it queries by table name with
// the database handle bound at construction, hiding the per-table Searcher and
// the SQL backend. The default implementation uses Postgres full-text search;
// an Elasticsearch/OpenSearch backend can implement Service later.
type Service interface {
	Search(ctx context.Context, table, query string, limit int) ([]Result, error)
}

type pgService struct {
	db DBTX
}

// NewService returns the default Postgres-FTS-backed search Service.
func NewService(db DBTX) Service {
	return &pgService{db: db}
}

func (s *pgService) Search(ctx context.Context, table, query string, limit int) ([]Result, error) {
	sr, err := NewSearcher(table)
	if err != nil {
		return nil, err
	}
	return sr.Search(ctx, s.db, query, limit)
}
