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
	rootCmd := &cobra.Command{
    Use:   "pxeserver",
    Short: "A server to PXE boot machines over the network",
    Long: "Built on top of Pixiecore with added templating, see https://github.com/danderson/netboot/blob/master/pixiecore/README.booting.md",
  }
	bootCmd := &cobra.Command{
    Use:   "boot",
    Short: "Start listening for PXE boot requests",
    Run: func(cmd *cobra.Command, args []string) {
  		execute(cfgFile)
    },
  }
  bootCmd.Flags().StringVar(&cfgFile, "config", "", "config file")

  rootCmd.AddCommand(bootCmd)

  if err := rootCmd.Execute(); err != nil {
    log.Fatal(err)
  }
}

func execute(configPath string) {
	var logSync sync.Mutex
	logFunc := func(subsys, msg string) {
		logSync.Lock()
		defer logSync.Unlock()
		fmt.Fprintf(os.Stderr, "[%s] %s\n", subsys, msg)
	}

	configFile, err := os.Open(configPath)
	if err != nil {
		log.Fatal(err)
	}
	defer configFile.Close()

	server := pxeserver.Server{
		// TODO: address flag
		Address: "0.0.0.0",
		Config: configFile,
		LogFunc: logFunc,
		// TODO: debug flag
		// TODO: DHCP nobind flag
		DHCPNoBind:       true,
	}
	fmt.Println(server.Serve())
}
