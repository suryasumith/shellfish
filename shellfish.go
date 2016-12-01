/*package shellfish contains code for computing the splashback shells of
halos in N-body simulations.*/
package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

	"github.com/phil-mansfield/shellfish/cmd"
	"github.com/phil-mansfield/shellfish/cmd/env"
	"github.com/phil-mansfield/shellfish/version"
)

var helpStrings = map[string]string{
	"id":    `Type "shellfish help" for basic information on invoking the id tool.

The id tool reads halo catalogs and finds the IDs of halos that correspond to
some user-specified range in either ID or mass space. It will automatically
throw out (R200m-identified) subhalos if asked, and can also return the IDs of
the (R200m-identified) subhalos of every host.

For a documented example of a config file used by the id tool, type:

     shellfish help id.config

The id tool takes no input from stdin.

The id tool prints the following catalogs to stdout:

Column 0 - ID:   The halo's catalog ID.
Column 1 - Snap: Index of the halo's snapshot.

If ExclusionStrategy = neighbor (i.e. if you want to find subhalos)

Column 0 - ID: The subhalo's catalog ID.
Column 1 - Snap: Index of the halo's snapshot`,

	"tree":  `Mode specifcations will be documented in version 1.0.`,
	"coord": `Mode specifcations will be documented in version 1.0.`,
	"prof": `Mode specifcations will be documented in version 1.0.`,
	"shell": `Mode specifcations will be documented in version 1.0.`,
	"stats": `Mode specifcations will be documented in version 1.0.`,

	"config":       new(cmd.GlobalConfig).ExampleConfig(),
	"id.config":    cmd.ModeNames["id"].ExampleConfig(),
	"tree.config":  cmd.ModeNames["tree"].ExampleConfig(),
	"coord.config": cmd.ModeNames["coord"].ExampleConfig(),
	"prof.config":  cmd.ModeNames["prof"].ExampleConfig(),
	"shell.config": cmd.ModeNames["shell"].ExampleConfig(),
	"stats.config": cmd.ModeNames["stats"].ExampleConfig(),
}

var modeDescriptions = `The best way to learn how to use shellfish is the tutorial on its github page:
https://github.com/phil-mansfield/shellfish/blob/master/doc/tutorial.md

The different tools in the Shellfish toolchain are:

    shellfish id     [____.id.config]    [flags]
    shellfish tree   [____.tree.config]  [flags]
    shellfish coord  [____.coord.config] [flags]
    shellfish prof   [____.prof.config]  [flags]
    shellfish shell  [____.shell.config] [flags]
    shellfish stats  [____.stats.config] [flags]

Each tool takes the name of a tool-specific config file. Without them, a
default set of variables will be used. You can also specify config variables
through command line flags of the form

    shellfish id --IDs "0, 1, 2, 3, 4, 5" --IDType "M200m"

If you supply both a config file and flags and the two give different values to
the same variable, the command line value will be used.

For documented example config files, type any of:

    shellfish help [ id.config | prof.config |shell.config |
                     stats.config | tree.config ]

In addition to any arguments passed at the command line, before calling
Shellfish rountines you will need to specify a "global" config file (it
has the file ending ".config"). Do this by setting the $SHELLFISH_GLOBAL_CONFIG
environment variable. For a documented global config file, type

    shellfish help config

The Shellfish tools expect an input catalog through stdin and will return an
output catalog through standard out. (The only exception is the id tool, which
doesn't take any input thorugh stdin) This means that you will generally invoke
shellfish as a series of piped commands. E.g:

    shellfish id example.id.config | shellfish coord | shellfish shell    

For more information on the input and output that a given tool expects, type
any of:

    shellfish help [ id | tree | coord | prof | shell | stats ]`

func main() {
	args := os.Args
	if len(args) <= 1 {
		fmt.Fprintf(
			os.Stderr, "I was not supplied with a mode.\nFor help, type "+
				"'./shellfish help'.\n",
		)
		os.Exit(1)
	}

	switch args[1] {
	case "help":
		switch len(args) - 2 {
		case 0:
			fmt.Println(modeDescriptions)
		case 1:
			text, ok := helpStrings[args[2]]
			if !ok {
				fmt.Printf("I don't recognize the help target '%s'\n", args[2])
			} else {
				fmt.Println(text)
			}
		case 2:
			fmt.Println("The help mode can only take a single argument.")
		}
		os.Exit(0)
		// TODO: Implement the help command.
	case "version":
		fmt.Printf("Shellfish version %s\n", version.SourceVersion)
		os.Exit(0)
	case "hello":
		fmt.Printf("Hello back at you! Installation was successful.\n")
		os.Exit(0)
	}

	mode, ok := cmd.ModeNames[args[1]]
	
	if !ok {
		fmt.Fprintf(
			os.Stderr, "You passed me the mode '%s', which I don't "+
				"recognize.\nFor help, type './shellfish help'\n", args[1],
		)
		fmt.Println("Shellfish terminating.")
		os.Exit(1)
	}

	var lines []string
	switch args[1] {
	case "tree", "coord", "prof", "shell", "stats":
		var err error
		lines, err = stdinLines()
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			fmt.Println("Shellfish terminating.")
			os.Exit(1)
		}

		if len(lines) == 0 {
			return
		} else if len(lines) == 1 && len(lines[0]) >= 9 &&
			lines[0][:9] == "Shellfish" {
			fmt.Println(lines[0])
			os.Exit(1)
		}
	}
	
	flags := getFlags(args)
	config, ok := getConfig(args)
	gConfigName, gConfig, err := getGlobalConfig(args)
	if err != nil {
		log.Printf("Error running mode %s:\n%s\n", args[1], err.Error())
		fmt.Println("Shellfish terminating.")
		os.Exit(1)
	}
	
	if ok {
		if err = mode.ReadConfig(config); err != nil {
			log.Printf("Error running mode %s:\n%s\n", args[1], err.Error())
			fmt.Println("Shellfish terminating.")
			os.Exit(1)
		}
	} else {
		if err = mode.ReadConfig(""); err != nil {
			log.Printf("Error running mode %s:\n%s\n", args[1], err.Error())
			fmt.Println("Shellfish terminating.")
			os.Exit(1)
		}
	}

	if err = checkMemoDir(gConfig.MemoDir, gConfigName); err != nil {
		log.Printf("Error running mode %s:\n%s\n", args[1], err.Error())
		fmt.Println("Shellfish terminating.")
		os.Exit(1)
	}
	
	e := &env.Environment{MemoDir: gConfig.MemoDir}
	err = initCatalogs(gConfig, e)
	if err != nil {
		log.Printf("Error running mode %s:\n%s\n", args[1], err.Error())
		fmt.Println("Shellfish terminating.")
		os.Exit(1)
	}
	
	err = initHalos(args[1], gConfig, e)
	if err != nil {
		log.Printf("Error running mode %s:\n%s\n", args[1], err.Error())
		fmt.Println("Shellfish terminating.")
		os.Exit(1)
	}
	
	out, err := mode.Run(flags, gConfig, e, lines)
	if err != nil {
		log.Printf("Error running mode %s:\n%s\n", args[1], err.Error())
		fmt.Println("Shellfish terminating.")
		os.Exit(1)
	}

	for i := range out {
		fmt.Println(out[i])
	}
}

// stdinLines reads stdin and splits it into lines.
func stdinLines() ([]string, error) {
	bs, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf(
			"Error reading stdin: %s.", err.Error(),
		)
	}
	text := string(bs)
	lines := strings.Split(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, nil
}

// getFlags reutrns the flag tokens from the command line arguments.
func getFlags(args []string) []string {
	return args[1 : len(args)-1-configNum(args)]
}

// getGlobalConfig returns the name of the base config file from the command
// line arguments.
func getGlobalConfig(args []string) (string, *cmd.GlobalConfig, error) {
	name := os.Getenv("SHELLFISH_GLOBAL_CONFIG")
	if name != "" {
		if configNum(args) > 1 {
			return "", nil, fmt.Errorf("$SHELLFISH_GLOBAL_CONFIG has been " +
				"set, so you may only pass a single config file as a " +
				"parameter.")
		}

		config := &cmd.GlobalConfig{}
		err := config.ReadConfig(name)
		if err != nil {
			return "", nil, err
		}
		return name, config, nil
	}

	switch configNum(args) {
	case 0:
		return "", nil, fmt.Errorf("No config files provided in command " +
			"line arguments.")
	case 1:
		name = args[len(args)-1]
	case 2:
		name = args[len(args)-2]
	default:
		return "", nil, fmt.Errorf("Passed too many config files as arguments.")
	}

	config := &cmd.GlobalConfig{}
	err := config.ReadConfig(name)
	if err != nil {
		return "", nil, err
	}
	return name, config, nil
}

// getConfig return the name of the mode-specific config file from the command
// line arguments.
func getConfig(args []string) (string, bool) {
	if os.Getenv("SHELLFISH_GLOBAL_CONFIG") != "" && configNum(args) == 1 {
		return args[len(args)-1], true
	} else if os.Getenv("SHELLFISH_GLOBAL_CONFIG") == "" &&
		configNum(args) == 2 {

		return args[len(args)-1], true
	}
	return "", false
}

// configNum returns the number of configuration files at the end of the
// argument list (up to 2).
func configNum(args []string) int {
	num := 0
	for i := len(args) - 1; i >= 0; i-- {
		if isConfig(args[i]) {
			num++
		} else {
			break
		}
	}
	return num
}

// isConfig returns true if the fiven string is a config file name.
func isConfig(s string) bool {
	return len(s) >= 7 && s[len(s)-7:] == ".config"
}

// cehckMemoDir checks whether the given MemoDir corresponds to a GlobalConfig
// file with the exact same variables. If not, a non-nil error is returned.
// If the MemoDir does not have an associated GlobalConfig file, the current
// one will be copied in.
func checkMemoDir(memoDir, configFile string) error {
	memoConfigFile := path.Join(memoDir, "memo.config")

	if _, err := os.Stat(memoConfigFile); err != nil {
		// File doesn't exist, directory is clean.
		err = copyFile(memoConfigFile, configFile)
		return err
	}

	config, memoConfig := &cmd.GlobalConfig{}, &cmd.GlobalConfig{}
	if err := config.ReadConfig(configFile); err != nil {
		return err
	}
	if err := memoConfig.ReadConfig(memoConfigFile); err != nil {
		return err
	}

	if !configEqual(config, memoConfig) {
		return fmt.Errorf("The variables in the config file '%s' do not "+
			"match the varables used when creating the MemoDir, '%s.' These "+
			"variables can be compared by inspecting '%s' and '%s'",
			configFile, memoDir, configFile, memoConfigFile,
		)
	}
	return nil
}

// copyFile copies a file from src to dst.
func copyFile(dst, src string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	return dstFile.Sync()
}

func configEqual(m, c *cmd.GlobalConfig) bool {
	// Well, equal up to the variables that actually matter.
	// (i.e. changing something like Threads shouldn't flush the memoization
	// buffer. Otherwise, I'd just use reflection.)
	return c.Version == m.Version &&
		c.SnapshotFormat == m.SnapshotFormat &&
		c.SnapshotType == m.SnapshotType &&
		c.HaloDir == m.HaloDir &&
		c.HaloType == m.HaloType &&
		c.TreeDir == m.TreeDir &&
		c.MemoDir == m.MemoDir && // (this is impossible)
		int64sEqual(c.BlockMins, m.BlockMins) &&
		int64sEqual(c.BlockMaxes, m.BlockMaxes) &&
		c.SnapMin == m.SnapMin &&
		c.SnapMax == m.SnapMax &&
		stringsEqual(c.SnapshotFormatMeanings, m.SnapshotFormatMeanings) &&
		c.HaloPositionUnits == m.HaloPositionUnits &&
		c.HaloMassUnits == m.HaloMassUnits &&
		int64sEqual(c.HaloValueColumns, m.HaloValueColumns) &&
		stringsEqual(c.HaloValueNames, m.HaloValueNames) &&
		c.Endianness == m.Endianness
}

func int64sEqual(xs, ys []int64) bool {
	if len(xs) != len(ys) {
		return false
	}
	for i := range xs {
		if xs[i] != ys[i] {
			return false
		}
	}
	return true
}

func stringsEqual(xs, ys []string) bool {
	if len(xs) != len(ys) {
		return false
	}
	for i := range xs {
		if xs[i] != ys[i] {
			return false
		}
	}
	return true
}

func initHalos(
	mode string, gConfig *cmd.GlobalConfig, e *env.Environment,
) error {
	switch mode {
	case "shell", "stats", "prof":
		return nil
	}

	switch gConfig.HaloType {
	case "nil":
		return fmt.Errorf("You may not use nil as a HaloType for the "+
			"mode '%s.'\n", mode)
	case "Text":
		return e.InitTextHalo(&gConfig.HaloInfo)
		if gConfig.TreeType != "consistent-trees" {
			return fmt.Errorf("You're trying to use the '%s' TreeType with " +
				"the 'Text' HaloType.")
		}
	}
	if gConfig.TreeType == "nil" {
		return fmt.Errorf("You may not use nil as a TreeType for the "+
			"mode '%s.'\n", mode)
	}

	panic("Impossible")
}

func initCatalogs(gConfig *cmd.GlobalConfig, e *env.Environment) error {
	switch gConfig.SnapshotType {
	case "gotetra":
		return e.InitGotetra(&gConfig.ParticleInfo, gConfig.ValidateFormats)
	case "LGadget-2":
		return e.InitLGadget2(&gConfig.ParticleInfo, gConfig.ValidateFormats)
	case "ARTIO":
		return e.InitARTIO(&gConfig.ParticleInfo, gConfig.ValidateFormats)
	}
	panic("Impossible.")
}
