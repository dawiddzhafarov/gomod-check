package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	ver "github.com/hashicorp/go-version"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/mod/modfile"
)

const (
	goModFile   = "go.mod"
	entryLength = 60
)

type config struct {
	maxVersionsFlag  int
	filterFlag       string
	incompatibleFlag bool
	terminalWidth    int
}

var (
	blue   = color.New(color.FgBlue).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	green  = color.New(color.FgGreen).SprintFunc()
	red    = color.New(color.FgGreen).SprintFunc()
	cfg    = config{}
)

func init() {
	flag.IntVar(&cfg.maxVersionsFlag, "max-versions", 10, "Specify how many versions to display.")                                                                                                                                                                   // validate?
	flag.StringVar(&cfg.filterFlag, "filter", "major,minor,patch", "Filter out the version types to display, available values: `major`, `minor`, `patch`. In order to use two filters, separate them with a comma (,). By default, all version types are included.") // validate, valid values: minor,major,patch
	flag.BoolVar(&cfg.incompatibleFlag, "show-incompatible", false, "Show incompatible versions, by default disabled.")                                                                                                                                              // validate, valid values: minor,major,patch
	flag.Parse()
	if cfg.maxVersionsFlag == 5 {
		log.Fatal("Oj")
	}
	if err := validateFlagValues(); err != nil {
		log.Fatal(err)
	}
}

func validateFlagValues() error {
	if cfg.maxVersionsFlag <= 0 || cfg.maxVersionsFlag > 1000 {
		return fmt.Errorf("`max-version` can be between 1 and 1000")
	}
	if len(cfg.filterFlag) != 0 && (!strings.Contains(cfg.filterFlag, "major") || !strings.Contains(cfg.filterFlag, "minor") ||
		!strings.Contains(cfg.filterFlag, "patch")) {
		return fmt.Errorf("`filter` flag can be made up only from `major`, `minor` and `patch` values")
	}

	return nil
}

func main() {

	fd := int(os.Stdin.Fd())
	width, _, err := terminal.GetSize(fd)
	fmt.Println(width)

	versionSpace := width - entryLength
	fmt.Println(versionSpace)

	versionNumber := versionSpace / 10 // TODO HANDLE VISUALISATION
	fmt.Println(versionNumber)

	// Read in go mod file
	data, err := os.ReadFile("./" + goModFile)
	if err != nil {
		panic(err)
	}

	// Parse file
	file, err := modfile.Parse(goModFile, data, nil)
	if err != nil {
		panic(err)
	}

	var mods Mods
	urlLength := 0
	curVersionLength := 0
	latestVersionLength := 0

	// Loop through required mods and exclude indirect ones
	for _, r := range file.Require {
		if r.Indirect {
			continue
		}

		// Check if mod is on the most current version
		mod, err := NewMod(r.Mod.Path, r.Mod.Version)
		if err != nil {
			continue
		}

		// Do nothing if mod is current
		if mod.Status == "current" {
			continue
		}

		// Check url length for option padding
		if urlLength < len(mod.Path) {
			urlLength = len(mod.Path)
		}
		if curVersionLength < len(mod.CurrentVersion.cleanString()) {
			curVersionLength = len(mod.CurrentVersion.cleanString())
		}
		if latestVersionLength < len(mod.AvailableVersions[0].cleanString()) {
			latestVersionLength = len(mod.AvailableVersions[0].cleanString())
		}

		mods = append(mods, *mod)
	}

	// Sort mods by status
	sort.Sort(mods)

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"#", "Dependency", "Current Version", "Available Versions"})
	t.SetColumnConfigs([]table.ColumnConfig{
		{
			Name:     "#",
			WidthMax: 3,
			Align:    text.AlignCenter,
		},
		{
			Name:     "Dependency",
			WidthMax: 30,
			Align:    text.AlignCenter,
		},
		{
			Name:     "Current Version",
			WidthMax: 17,
			Align:    text.AlignCenter,
		},
		{
			Name:     "Available Versions",
			WidthMax: versionSpace,
		},
	})

	for i, mod := range mods {
		var newVersions []string
		for _, availableVer := range mod.AvailableVersions {
			currentVersion, err := ver.NewVersion(mod.CurrentVersion.original)
			if err != nil {
				panic(err)
			}
			availableVersion, err := ver.NewVersion(availableVer.original)
			if availableVersion.GreaterThan(currentVersion) {
				if v := filterVersions(availableVer, cfg.incompatibleFlag, cfg.filterFlag); v != "" {
					newVersions = append(newVersions, v)
				}
			}
		}
		if len(newVersions) != 0 {
			unwrapRows(i, mod.Path, mod.CurrentVersion.original, versionNumber, newVersions, t)
		}
	}
	if t.Length() != 0 {
		t.AppendSeparator()
		t.Render()
	} else {
		fmt.Println("There are no newer versions that fulfill provided requirements.")
		fmt.Printf("Filter: %s", cfg.filterFlag)
	}
}

func unwrapRows(i int, path string, curVersion string, num int, versions []string, t table.Writer) {
	length := len(versions)
	if length > num {
		t.AppendRow([]interface{}{i + 1, blue(path), curVersion, strings.Join(versions[:num], ", ")})
		for length > num {
			length -= num
			versions = versions[num:]
			if len(versions) < num {
				t.AppendRow([]interface{}{"", "", "", strings.Join(versions, ", ")})
				t.AppendSeparator()
			} else {
				t.AppendRow([]interface{}{"", "", "", strings.Join(versions[:num], ", ")})
			}
		}
	} else {
		t.AppendRow([]interface{}{i + 1, blue(path), curVersion, strings.Join(versions, ", ")})
		t.AppendSeparator()
	}
}

func filterVersions(ver version, showIncompatible bool, filter string) string {
	if !showIncompatible && ver.incompatible {
		return ""
	}

	if filter == "" {
		return colorVersion(ver.status, ver.original)
	}

	filters := strings.Split(filter, ",")
	for _, fil := range filters {
		if strings.Contains(ver.status, fil) {
			return colorVersion(ver.status, ver.original)
		}
	}

	return ""
}

func colorVersion(status string, version string) string {
	switch status {
	case "minor":
		return yellow(version)
	case "major":
		return red(version)
	case "patch":
		return green(version)
	default:
		return blue(version)
	}
}
