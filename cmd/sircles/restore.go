package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sorintlab/sircles/config"
	"github.com/sorintlab/sircles/db"
	"github.com/sorintlab/sircles/eventstore"
	slog "github.com/sorintlab/sircles/log"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"
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
		return errors.WithMessage(err, fmt.Sprintf("error parsing configuration file %s", configFile))
	}

	if c.Debug {
		slog.SetLevel(zapcore.DebugLevel)
	}

	if c.EventStore.Type == "" {
		return errors.New("no eventstore type specified")
	}
	if c.EventStore.Type != "sql" {
		return errors.Errorf("unknown eventstore type: %q", c.EventStore.Type)
	}
	if c.EventStore.DB.Type == "" {
		return errors.New("no eventstore db type specified")
	}

	switch c.EventStore.DB.Type {
	case db.Postgres:
	case db.Sqlite3:
	default:
		return errors.Errorf("unsupported eventstore db type: %s", c.EventStore.DB.Type)
	}

	esLnType := getLNtype(&c.EventStore.DB)
	_, esNf, err := getListenerNotifierFactories(esLnType, &c.EventStore.DB)

	esDB, err := db.NewDB(c.EventStore.DB.Type, c.EventStore.DB.ConnString)
	if err != nil {
		return err
	}

	// Populate/migrate esdb
	if err := esDB.Migrate("eventstore", eventstore.Migrations); err != nil {
		return err
	}

	es := eventstore.NewEventStore(esDB, esNf)

	f, err := os.Open(dumpFile)
	if err != nil {
		return err
	}

	dec := json.NewDecoder(f)

	events := []*eventstore.StoredEvent{}
	for {
		var event *eventstore.StoredEvent
		err := dec.Decode(&event)
		if err != nil {
			return err
		}

		events = append(events, event)

		hasMore := dec.More()
		if len(events) >= restoreBatchSize || !hasMore {
			if err := es.RestoreEvents(events); err != nil {
				return err
			}
			events = []*eventstore.StoredEvent{}
		}
		if !hasMore {
			break
		}
	}

	return nil
}
