package eventhandler

import (
	"database/sql"
	"fmt"
	"path/filepath"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"
	"github.com/sorintlab/sircles/aggregate"
	"github.com/sorintlab/sircles/command/commands"
	"github.com/sorintlab/sircles/common"
	"github.com/sorintlab/sircles/db"
	"github.com/sorintlab/sircles/eventstore"
	"github.com/sorintlab/sircles/models"
	"github.com/sorintlab/sircles/util"
)

const (
	dbName = "drth.db"
)

func newDB(dataDir string) (*db.DB, error) {
	return db.NewDB("sqlite3", filepath.Join(dataDir, dbName))
}

type DeletedRoleTensionHandler struct {
	dataDir      string
	es           *eventstore.EventStore
	uidGenerator common.UIDGenerator
}

func NewDeletedRoleTensionHandler(dataDir string, es *eventstore.EventStore, uidGenerator common.UIDGenerator) (*DeletedRoleTensionHandler, error) {
	ldb, err := newDB(dataDir)
	if err != nil {
		return nil, err
	}
	defer ldb.Close()

	err = ldb.Do(func(tx *db.Tx) error {
		return tx.Do(func(tx *db.WrappedTx) error {
			for _, stmt := range drthDBCreateStmts {
				if _, err := tx.Exec(stmt); err != nil {
					return errors.WithMessage(err, "create failed")
				}
			}
			return nil
		})
	})

	return &DeletedRoleTensionHandler{
		dataDir:      dataDir,
		es:           es,
		uidGenerator: uidGenerator,
	}, err
}

func (h *DeletedRoleTensionHandler) Name() string {
	return "deletedRoleTensionHandler"
}

func (h *DeletedRoleTensionHandler) updateTensions(ldb *db.DB) error {
	var tensions map[util.ID]int64
	err := ldb.Do(func(tx *db.Tx) error {
		var err error
		tensions, err = h.findTensions(tx)
		return err
	})
	if err != nil {
		return err
	}
	for tid, version := range tensions {
		tr := aggregate.NewTensionRepository(h.es, h.uidGenerator)
		t, err := tr.Load(tid)
		if err != nil {
			return err
		}
		correlationID := h.uidGenerator.UUID("")
		causationID := h.uidGenerator.UUID("")
		command := commands.NewCommand(commands.CommandTypeChangeTensionRole, correlationID, causationID, util.NilID, commands.NewCommandChangeTensionRole(nil, version))

		events, err := t.HandleCommand(command)
		if err != nil {
			return err
		}

		groupID := h.uidGenerator.UUID("")
		eventsData, err := eventstore.GenEventData(events, &correlationID, &causationID, &groupID, nil)
		if err != nil {
			return err
		}
		if _, err = h.es.WriteEvents(eventsData, t.AggregateType().String(), t.ID(), t.Version()); err != nil {
			return err
		}
	}
	return nil
}

func (h *DeletedRoleTensionHandler) HandleEvents() error {
	log.Debugf("eh handleEvents")
	ldb, err := newDB(h.dataDir)
	if err != nil {
		return err
	}
	defer ldb.Close()

	for {
		var n int
		err := ldb.Do(func(tx *db.Tx) error {
			var err error
			n, err = h.updateSnapshot(tx)
			return err
		})
		if err != nil {
			return err
		}
		if n == 0 {
			break
		}
	}

	return h.updateTensions(ldb)
}

func (h *DeletedRoleTensionHandler) updateSnapshot(tx *db.Tx) (int, error) {
	log.Debugf("updateSnapshot")

	sn, err := h.SequenceNumber(tx)
	if err != nil {
		return 0, err
	}
	log.Debugf("sn: %d", sn)

	events, err := h.es.GetAllEvents(sn+1, 100)
	if err != nil {
		return 0, err
	}

	for _, e := range events {
		if err := h.handleEvent(tx, e); err != nil {
			return 0, err
		}
	}

	sn, err = h.SequenceNumber(tx)
	if err != nil {
		return 0, err
	}
	log.Debugf("sn: %d", sn)

	return len(events), nil
}

func (h *DeletedRoleTensionHandler) handleEvent(tx *db.Tx, event *eventstore.StoredEvent) error {
	log.Debugf("event: %v", event)

	data, err := event.UnmarshalData()
	if err != nil {
		return err
	}

	switch event.EventType {
	case eventstore.EventTypeRoleCreated:

	case eventstore.EventTypeRoleUpdated:
		data := data.(*eventstore.EventRoleUpdated)
		switch data.RoleType {
		case models.RoleTypeCircle:
			if err := h.deleteDeletedRole(tx, data.RoleID); err != nil {
				return err
			}
		default:
			if err := h.insertDeletedRole(tx, data.RoleID); err != nil {
				return err
			}
		}

	case eventstore.EventTypeRoleDeleted:
		data := data.(*eventstore.EventRoleDeleted)
		if err := h.insertDeletedRole(tx, data.RoleID); err != nil {
			return err
		}

	case eventstore.EventTypeTensionCreated:
		data := data.(*eventstore.EventTensionCreated)
		tensionID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return err
		}
		if err := h.updateTension(tx, tensionID, event.Version, data.RoleID); err != nil {
			return err
		}

	case eventstore.EventTypeTensionRoleChanged:
		data := data.(*eventstore.EventTensionRoleChanged)
		tensionID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return err
		}
		if err := h.updateTension(tx, tensionID, event.Version, data.RoleID); err != nil {
			return err
		}

	case eventstore.EventTypeTensionClosed:
		tensionID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return err
		}
		if err := h.deleteTension(tx, tensionID); err != nil {
			return err
		}
	}

	// for every tension event update tension version if tension exists
	if event.Category == eventstore.TensionAggregate.String() {
		tensionID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return err
		}
		if err := h.updateTensionVersion(tx, tensionID, event.Version); err != nil {
			return err
		}
	}

	if err := h.updateSequenceNumber(tx, event.SequenceNumber); err != nil {
		return err
	}

	return nil
}

// DeletedRoleTensionHandler snapshot db
var drthDBCreateStmts = []string{
	"create table if not exists deletedrole (id uuid, PRIMARY KEY (id))",
	"create table if not exists tension (id uuid, version bigint, roleid uuid, PRIMARY KEY (id))",
	"create table if not exists sequencenumber (sequencenumber bigint)",
}

var (
	sb = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

	deletesRoleSelect = sb.Select("id").From("deletedrole")
	deletesRoleInsert = sb.Insert("deletedrole").Columns("id")
	deletesRoleDelete = sb.Delete("deletedrole")
	deletesRoleUpdate = sb.Update("deletedrole")

	tensionSelect = sb.Select("id", "roleid", "version").From("tension")
	tensionInsert = sb.Insert("tension").Columns("id", "version", "roleid")
	tensionDelete = sb.Delete("tension")
	tensionUpdate = sb.Update("tension")

	sequenceNumberSelect = sb.Select("sequencenumber").From("sequencenumber")
	sequenceNumberInsert = sb.Insert("sequencenumber").Columns("sequencenumber")
	sequenceNumberDelete = sb.Delete("sequencenumber")
)

func (h *DeletedRoleTensionHandler) findTensions(tx *db.Tx) (map[util.ID]int64, error) {
	q, args, err := sb.Select("t.id, t.version").From("tension as t").Join("deletedrole as r on t.roleid = r.id").ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build query")
	}

	tensions := map[util.ID]int64{}
	err = tx.Do(func(tx *db.WrappedTx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return errors.Wrap(err, "failed to execute query")
		}
		for rows.Next() {
			var tid util.ID
			var version int64
			if err := rows.Scan(&tid, &version); err != nil {
				return errors.Wrap(err, "failed to scan rows")
			}
			tensions[tid] = version
		}
		if err := rows.Err(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, errors.WithMessage(err, "failed to find tension assigned to deleted role")
	}
	return tensions, nil
}

func (h *DeletedRoleTensionHandler) insertDeletedRole(tx *db.Tx, id util.ID) error {
	if err := h.deleteDeletedRole(tx, id); err != nil {
		return err
	}

	q, args, err := deletesRoleInsert.Values(id).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("failed to insert deleted role %s", id))
	}
	return nil
}

func (h *DeletedRoleTensionHandler) deleteDeletedRole(tx *db.Tx, id util.ID) error {
	q, args, err := deletesRoleDelete.Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("failed to delete deleted role %s", id))
	}
	return nil
}

func (h *DeletedRoleTensionHandler) updateTension(tx *db.Tx, id util.ID, version int64, roleID *util.ID) error {
	if err := h.deleteTension(tx, id); err != nil {
		return err
	}

	q, args, err := tensionInsert.Values(id, version, roleID).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("failed to insert tension %s", id))
	}
	return nil
}

func (h *DeletedRoleTensionHandler) deleteTension(tx *db.Tx, id util.ID) error {
	q, args, err := tensionDelete.Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("failed to delete tension %s", id))
	}
	return nil
}
func (h *DeletedRoleTensionHandler) updateTensionVersion(tx *db.Tx, id util.ID, version int64) error {
	q, args, err := sb.Update("tension").Set("version", version).Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("failed to update tension %s version", id))
	}
	return nil
}

func (h *DeletedRoleTensionHandler) SequenceNumber(tx *db.Tx) (int64, error) {
	var sn int64
	err := tx.Do(func(tx *db.WrappedTx) error {
		return tx.QueryRow("select sequencenumber from sequencenumber limit 1").Scan(&sn)
	})
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return sn, nil
}

func (h *DeletedRoleTensionHandler) updateSequenceNumber(tx *db.Tx, sn int64) error {
	q, args, err := sequenceNumberDelete.ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}
	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("failed to delete sequencenumber: %v", sn))
	}

	q, args, err = sequenceNumberInsert.Values(sn).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("failed to insert sequencenumber: %v", sn))
	}
	return nil
}
