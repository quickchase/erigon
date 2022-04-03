package commands

import (
	"context"
	"encoding/binary"
	"fmt"
	"path/filepath"

	"github.com/ledgerwatch/erigon-lib/kv"
	kv2 "github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/erigon/cmd/utils"
	"github.com/ledgerwatch/erigon/common/dbutils"
	"github.com/ledgerwatch/erigon/internal/debug"
	"github.com/ledgerwatch/erigon/migrations"
	"github.com/ledgerwatch/log/v3"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "integration",
	Short: "long and heavy integration tests for Erigon",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if err := utils.SetupCobra(cmd); err != nil {
			panic(err)
		}
		if chaindata == "" {
			chaindata = filepath.Join(datadir, "chaindata")
		}
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		defer utils.StopDebug()
	},
}

func RootCommand() *cobra.Command {
	utils.CobraFlags(rootCmd, append(debug.Flags, utils.MetricFlags...))
	return rootCmd
}

func openDB(path string, logger log.Logger, applyMigrations bool) kv.RwDB {
	label := kv.ChainDB
	db := openKV(label, logger, path, false)
	db.View(context.Background(), func(tx kv.Tx) error {
		fmt.Printf("--headers\n")
		_ = tx.ForAmount(kv.Headers, dbutils.EncodeBlockNumber(15521071-4), 8, func(k, v []byte) error {
			fmt.Printf("%d, %d, %x\n", binary.BigEndian.Uint64(k), len(v), k[8:])
			return nil
		})
		fmt.Printf("--bodies\n")
		_ = tx.ForAmount(kv.BlockBody, dbutils.EncodeBlockNumber(15521071-4), 8, func(k, v []byte) error {
			fmt.Printf("%d, %d\n", binary.BigEndian.Uint64(k), len(v))
			return nil
		})
		fmt.Printf("--canonical\n")
		_ = tx.ForAmount(kv.HeaderCanonical, dbutils.EncodeBlockNumber(15521071-4), 8, func(k, v []byte) error {
			fmt.Printf("%d, %x\n", binary.BigEndian.Uint64(k), v)
			return nil
		})
		return nil
	})
	if applyMigrations {
		has, err := migrations.NewMigrator(label).HasPendingMigrations(db)
		if err != nil {
			panic(err)
		}
		if has {
			log.Info("Re-Opening DB in exclusive mode to apply DB migrations")
			db.Close()
			db = openKV(label, logger, path, true)
			if err := migrations.NewMigrator(label).Apply(db, datadir); err != nil {
				panic(err)
			}
			db.Close()
			db = openKV(label, logger, path, false)
		}
	}
	return db
}

func openKV(label kv.Label, logger log.Logger, path string, exclusive bool) kv.RwDB {
	opts := kv2.NewMDBX(logger).Path(path).Label(label)
	if exclusive {
		opts = opts.Exclusive()
	}
	if databaseVerbosity != -1 {
		opts = opts.DBVerbosity(kv.DBVerbosityLvl(databaseVerbosity))
	}
	return opts.MustOpen()
}
