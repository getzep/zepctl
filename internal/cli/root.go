package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Version information set by goreleaser.
	version = "dev"
	commit  = "none"
	date    = "unknown"

	cfgFile string
)

var rootCmd = &cobra.Command{
	Use:   "zepctl",
	Short: "CLI for administering Zep projects",
	Long: `zepctl is a command-line interface for administering Zep projects
and improving the developer experience. It provides comprehensive access
to Zep's context engineering platform.`,
	SilenceUsage: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.zepctl/config.yaml)")
	rootCmd.PersistentFlags().StringP("api-key", "k", "", "API key for authentication")
	rootCmd.PersistentFlags().String("api-url", "", "API endpoint URL (uses SDK default if not set)")
	rootCmd.PersistentFlags().StringP("profile", "p", "", "Use specific profile")
	rootCmd.PersistentFlags().StringP("output", "o", "table", "Output format: table, json, yaml, wide")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")

	_ = viper.BindPFlag("api-key", rootCmd.PersistentFlags().Lookup("api-key"))
	_ = viper.BindPFlag("api-url", rootCmd.PersistentFlags().Lookup("api-url"))
	_ = viper.BindPFlag("profile", rootCmd.PersistentFlags().Lookup("profile"))
	_ = viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home + "/.zepctl")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	viper.SetEnvPrefix("ZEP")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		if viper.GetBool("verbose") {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}
}
