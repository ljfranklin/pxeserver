package main

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/ljfranklin/pxeserver"
	"github.com/spf13/cobra"
)

func main() {
	var cfgFile string
	var secretsFile string
	var host string
	var id string
	var field string
	rootCmd := &cobra.Command{
		Use:   "pxeserver",
		Short: "A server to PXE boot machines over the network",
		Long:  "Built on top of Pixiecore with added templating, see https://github.com/danderson/netboot/blob/master/pixiecore/README.booting.md",
	}
	bootCmd := &cobra.Command{
		Use:   "boot",
		Short: "Start listening for PXE boot requests",
		Run: func(cmd *cobra.Command, args []string) {
			executeBoot(bootArgs{
				ConfigPath:  cfgFile,
				SecretsPath: secretsFile,
			})
		},
	}
	secretsCmd := &cobra.Command{
		Use:   "secrets",
		Short: "Print generated secret to Stdout",
		Run: func(cmd *cobra.Command, args []string) {
			executeSecrets(secretsArgs{
				SecretsPath: secretsFile,
				Host:        host,
				ID:          id,
				Field:       field,
			})
		},
	}
	// TODO: document flags
	bootCmd.Flags().StringVar(&cfgFile, "config", "", "config file")
	bootCmd.Flags().StringVar(&secretsFile, "secrets", "", "secrets file")
	secretsCmd.Flags().StringVar(&cfgFile, "config", "", "config file")
	secretsCmd.Flags().StringVar(&secretsFile, "secrets", "", "secrets file")
	secretsCmd.Flags().StringVar(&host, "host", "", "host mac")
	secretsCmd.Flags().StringVar(&id, "id", "", "secret id")
	secretsCmd.Flags().StringVar(&field, "field", "", "secret field")

	rootCmd.AddCommand(bootCmd)
	rootCmd.AddCommand(secretsCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

type bootArgs struct {
	ConfigPath  string
	SecretsPath string
}

func executeBoot(args bootArgs) {
	var logSync sync.Mutex
	logFunc := func(subsys, msg string) {
		logSync.Lock()
		defer logSync.Unlock()
		fmt.Fprintf(os.Stderr, "[%s] %s\n", subsys, msg)
	}

	configFile, err := os.Open(args.ConfigPath)
	if err != nil {
		log.Fatal(err)
	}
	defer configFile.Close()

	server := pxeserver.Server{
		// TODO: address flag
		Address: "0.0.0.0",
		Config:  configFile,
		LogFunc: logFunc,
		// TODO: debug flag
		// TODO: DHCP nobind flag
		DHCPNoBind:  true,
		SecretsPath: args.SecretsPath,
	}
	fmt.Println(server.Serve())
}

type secretsArgs struct {
	SecretsPath string
	Host        string
	ID          string
	Field       string
}

func executeSecrets(args secretsArgs) {
	secrets, err := pxeserver.LoadLocalSecrets(args.SecretsPath, nil)
	if err != nil {
		log.Fatal(err)
	}
	result, err := secrets.GetField(args.Host, args.ID, args.Field)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result)
}
