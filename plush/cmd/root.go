package cmd

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/gobuffalo/plush"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "plush",
	Short: "A brief description of your application",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("you must pass in at least 1 plush file")
		}

		for _, a := range args {
			b, err := ioutil.ReadFile(a)
			if err != nil {
				return errors.WithStack(err)
			}
			err = plush.RunScript(string(b), plush.NewContext())
			if err != nil {
				return errors.WithStack(err)
			}

		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
