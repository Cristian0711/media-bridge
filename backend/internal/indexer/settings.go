package indexer

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// IndexerSetting persists per-indexer search preferences configured by admins.
// Settings are keyed by indexer name because that is what search results carry
// (IndexerItem.IndexerName), matching the Prowlarr catalog entry name.
type IndexerSetting struct {
	ID          uint   `gorm:"primaryKey" json:"-"`
	IndexerName string `gorm:"uniqueIndex;not null" json:"indexer_name"`
	// No GORM `default` tag: with a default, GORM omits the field from INSERT
	// when it is the zero value (false), so toggling an indexer to "include all"
	// would silently persist as freeleech-only. The application layer supplies
	// the freeleech-only default for unconfigured indexers (see freeleechPolicy).
	FreeleechOnly bool      `gorm:"not null" json:"freeleech_only"`
	CreatedAt     time.Time `json:"-"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SettingsRepository persists per-indexer settings.
type SettingsRepository interface {
	List(ctx context.Context) ([]IndexerSetting, error)
	Upsert(ctx context.Context, indexerName string, freeleechOnly bool) (*IndexerSetting, error)
}

type settingsRepository struct {
	db *gorm.DB
}

// NewSettingsRepository builds a GORM-backed per-indexer settings store.
func NewSettingsRepository(db *gorm.DB) SettingsRepository {
	return &settingsRepository{db: db}
}

func (r *settingsRepository) List(ctx context.Context) ([]IndexerSetting, error) {
	var out []IndexerSetting
	err := r.db.WithContext(ctx).Order("indexer_name ASC").Find(&out).Error
	return out, err
}

func (r *settingsRepository) Upsert(ctx context.Context, indexerName string, freeleechOnly bool) (*IndexerSetting, error) {
	setting := &IndexerSetting{IndexerName: strings.TrimSpace(indexerName), FreeleechOnly: freeleechOnly}
	// Explicit assignments (not AssignmentColumns) so the on-conflict UPDATE writes
	// the literal freeleechOnly value rather than reading it back from the proposed
	// insert row, which is robust regardless of GORM zero-value handling.
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "indexer_name"}},
			DoUpdates: clause.Assignments(map[string]any{
				"freeleech_only": freeleechOnly,
				"updated_at":     time.Now(),
			}),
		}).
		Create(setting).Error
	if err != nil {
		return nil, err
	}
	return setting, nil
}

// freeleechPolicy resolves whether an indexer must return freeleech-only results.
// Indexers without an explicit setting default to freeleech-only, preserving the
// conservative default the filter used before settings were configurable.
type freeleechPolicy struct {
	byName map[string]bool
}

func (p freeleechPolicy) freeleechOnly(indexerName string) bool {
	if p.byName != nil {
		if v, ok := p.byName[strings.ToLower(strings.TrimSpace(indexerName))]; ok {
			return v
		}
	}
	return true
}
