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

var dumpCmd = &cobra.Command{
	Use: "dump",
	Run: func(cmd *cobra.Command, args []string) {
		if err := dump(cmd, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(-1)
		}
	},
}

func init() {
	rootCmd.AddCommand(dumpCmd)

	dumpCmd.PersistentFlags().StringVar(&dumpFile, "dumpfile", "", "path to dump file")
}

func dump(cmd *cobra.Command, args []string) error {
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

	f, err := os.Create(dumpFile)
	if err != nil {
		return err
	}

	i := int64(1)
	for {
		events, err := es.GetAllEvents(i, 100)
		if err != nil {
			return err
		}
		if len(events) == 0 {
			break
		}
		i += 100

		for _, event := range events {
			log.Infof("sequencenumber: %d", event.SequenceNumber)
			eventj, err := json.Marshal(event)
			if err != nil {
				return errors.WithStack(err)
			}
			f.Write(eventj)
			f.Write([]byte("\n"))
			log.Infof("eventj: %s", eventj)
		}
	}
	f.Close()
	f.Sync()

	return nil
}
