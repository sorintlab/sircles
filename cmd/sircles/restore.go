package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sorintlab/sircles/config"
	"github.com/sorintlab/sircles/db"
	"github.com/sorintlab/sircles/eventstore"
	slog "github.com/sorintlab/sircles/log"
	"github.com/sorintlab/sircles/readdb"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	restoreBatchSize = 100
)

var restoreCmd = &cobra.Command{
	Use: "restore",
	Run: func(cmd *cobra.Command, args []string) {
		if err := restore(cmd, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(-1)
		}
	},
}

func init() {
	rootCmd.AddCommand(restoreCmd)

	restoreCmd.PersistentFlags().StringVar(&dumpFile, "dumpfile", "", "path to dump file")
}

func restore(cmd *cobra.Command, args []string) error {
	if configFile == "" {
		return errors.New("you should provide a config file path (-c option)")
	}
	if dumpFile == "" {
		return errors.New("you should provide a dump file path (--dumpfile option)")
	}

	c, err := config.Parse(configFile)
	if err != nil {
		return fmt.Errorf("error parsing configuration file %s: %v", configFile, err)
	}

	if c.Debug {
		slog.SetDebug(true)
	}

	if c.DB.Type == "" {
		return errors.New("no db type specified")
	}
	switch c.DB.Type {
	case "postgres":
	case "cockroachdb":
	case "sqlite3":
	default:
		return fmt.Errorf("unsupported db type: %s", c.DB.Type)
	}

	db, err := db.NewDB(c.DB.Type, c.DB.ConnString)
	if err != nil {
		return err
	}

	// Populate/migrate db
	if err := db.Migrate(); err != nil {
		return err
	}

	tx, err := db.NewTx()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		tx.Commit()
	}()

	f, err := os.Open(dumpFile)
	if err != nil {
		return err
	}

	dec := json.NewDecoder(f)

	events := eventstore.Events{}
	for {
		var event *eventstore.Event
		err := dec.Decode(&event)
		if err != nil {
			log.Fatal(err)
		}

		events = append(events, event)

		hasMore := dec.More()
		if len(events) >= restoreBatchSize || !hasMore {
			if err := applyEvents(db, events); err != nil {
				return err
			}
			events = eventstore.Events{}
		}
		if !hasMore {
			break
		}
	}

	return nil
}

func applyEvents(db *db.DB, events eventstore.Events) error {
	tx, err := db.NewTx()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		tx.Commit()
	}()

	es := eventstore.NewEventStore(tx)

	readDB, err := readdb.NewDBService(tx)
	if err != nil {
		return err
	}
	sequenceNumber := events[len(events)-1].SequenceNumber
	curSequenceNumber, err := es.RestoreEvents(events)
	if err != nil {
		return err
	}
	if sequenceNumber != curSequenceNumber {
		return errors.Errorf("expected sequence number: %d != writed event sequence number: %d", sequenceNumber, curSequenceNumber)
	}
	if err := readDB.ApplyEvents(events); err != nil {
		return err
	}
	return nil
}
