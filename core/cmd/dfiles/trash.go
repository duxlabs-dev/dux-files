package main

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/duxlabs-dev/dux-files/core/internal/fileops"

	"github.com/spf13/cobra"

	"github.com/duxlabs-dev/dux-go/trash"
)

var (
	trashDirFlag      string
	trashConflictFlag string
)

var trashCmd = &cobra.Command{
	Use:   "trash",
	Short: "Move files to the trash, list, restore or empty it",
}

var trashPutCmd = &cobra.Command{
	Use:   "put <path...>",
	Short: "Move files or directories into the trash",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		sources, err := absolutePaths(args)
		if err != nil {
			return err
		}
		return withSession(fileServices, func(s *session) error {
			result, err := runJobUnattended(s, "files.op.trash", map[string]any{"paths": sources}, "skip")
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(result)
			}
			for _, source := range sources {
				if result.succeeded(source) {
					fmt.Println(source)
				}
			}
			return result.err()
		})
	},
}

var trashListCmd = &cobra.Command{
	Use:   "list",
	Short: "List trashed items across every trash directory",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		entries, err := trashEntries()
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(entries)
		}
		if len(entries) == 0 {
			fmt.Println("Trash is empty")
			return nil
		}
		for _, e := range entries {
			marker := "F"
			if e.IsDir {
				marker = "D"
			}
			fmt.Printf("%s  %s  %s  %s\n", marker, e.DeletionDate, e.Name, e.OriginalPath)
		}
		return nil
	},
}

var trashCountCmd = &cobra.Command{
	Use:   "count",
	Short: "Print the number of trashed items",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return withSession(noServices, func(s *session) error {
			var out struct {
				Count int `json:"count"`
			}
			if err := s.call("trash.count", nil, &out); err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(out)
			}
			fmt.Println(out.Count)
			return nil
		})
	},
}

var trashRestoreCmd = &cobra.Command{
	Use:               "restore <name...>",
	Short:             "Restore trashed items to where they were deleted from",
	Long:              "Restore trashed items by the name 'dfiles trash list' shows. A conflict with an existing file keeps both unless --conflict says otherwise.",
	Args:              cobra.MinimumNArgs(1),
	ValidArgsFunction: completeTrashNames,
	RunE: func(_ *cobra.Command, args []string) error {
		action, err := conflictAction(trashConflictFlag)
		if err != nil {
			return err
		}
		byDir, order, err := trashDirsFor(slices.Compact(slices.Sorted(slices.Values(args))))
		if err != nil {
			return err
		}
		return withSession(fileServices, func(s *session) error {
			results := make([]jobResult, 0, len(order))
			var failed error
			for _, dir := range order {
				result, err := runJobUnattended(s, "files.op.restore", map[string]any{"names": byDir[dir], "trashDir": dir}, action)
				if err != nil {
					return err
				}
				results = append(results, result)
				failed = cmp.Or(failed, result.err())
			}
			if jsonOutput {
				return printJSON(results)
			}
			return failed
		})
	},
}

func trashDirsFor(names []string) (map[string][]string, []string, error) {
	if trashDirFlag != "" {
		return map[string][]string{trashDirFlag: names}, []string{trashDirFlag}, nil
	}
	entries, err := trashEntries()
	if err != nil {
		return nil, nil, err
	}
	home := fileops.HomeTrash()
	byDir := map[string][]string{}
	var order []string
	for _, name := range names {
		dir := ""
		for _, e := range entries {
			if e.Name != name {
				continue
			}
			if dir == "" || e.TrashDir == home {
				dir = e.TrashDir
			}
		}
		if dir == "" {
			return nil, nil, fmt.Errorf("%s is not in the trash", name)
		}
		if _, seen := byDir[dir]; !seen {
			order = append(order, dir)
		}
		byDir[dir] = append(byDir[dir], name)
	}
	return byDir, order, nil
}

var trashEmptyCmd = &cobra.Command{
	Use:   "empty",
	Short: "Delete every trashed item permanently",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return withSession(fileServices, func(s *session) error {
			result, err := runJobUnattended(s, "files.op.emptyTrash", map[string]any{}, "skip")
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(result)
			}
			return result.err()
		})
	},
}

func trashEntries() ([]trash.Entry, error) {
	var out struct {
		Entries []trash.Entry `json:"entries"`
	}
	err := withSession(noServices, func(s *session) error {
		return s.call("trash.list", nil, &out)
	})
	return out.Entries, err
}

func completeTrashNames(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	entries, err := trashEntries()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name+"\t"+e.OriginalPath)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	trashRestoreCmd.Flags().StringVar(&trashDirFlag, "trash-dir", "", "Trash directory holding the items (default: wherever the listing finds them, the home trash first)")
	trashRestoreCmd.Flags().StringVar(&trashConflictFlag, "conflict", "keep-both", "When the original path is taken: keep-both, skip or replace")
	trashCmd.AddCommand(trashPutCmd, trashListCmd, trashCountCmd, trashRestoreCmd, trashEmptyCmd)
}
