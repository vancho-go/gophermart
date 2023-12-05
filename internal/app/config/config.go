package config

import (
	"flag"
	"os"
)

type ServerConfig struct {
	ServerRunAddress     string
	DatabaseURI          string
	AccrualSystemAddress string
}

type serverConfigBuilder struct {
	serviceConfig ServerConfig
}

func newServiceConfigBuilder() *serverConfigBuilder {
	return &serverConfigBuilder{
		serviceConfig: ServerConfig{},
	}
}

func (sc *serverConfigBuilder) withServerRunAddress(serverRunAddress string) *serverConfigBuilder {
	sc.serviceConfig.ServerRunAddress = serverRunAddress
	return sc
}

func (sc *serverConfigBuilder) withDatabaseURI(databaseURI string) *serverConfigBuilder {
	sc.serviceConfig.DatabaseURI = databaseURI
	return sc
}

func (sc *serverConfigBuilder) withAccrualSystemAddress(accrualSystemAddress string) *serverConfigBuilder {
	sc.serviceConfig.AccrualSystemAddress = accrualSystemAddress
	return sc
}

func (sc *serverConfigBuilder) build() ServerConfig {
	return sc.serviceConfig
}

func BuildServer() (ServerConfig, error) {
	var (
		serverRunAddress     string
		databaseURI          string
		accrualSystemAddress string
	)

	flag.StringVar(&serverRunAddress, "a", "localhost:8080", "address:port to run server")
	flag.StringVar(&databaseURI, "d", "", "connection string for driver to establish connection to he DB")
	flag.StringVar(&accrualSystemAddress, "r", "", "address of the accrual calculation system")
	flag.Parse()

	if envServerRunAddress := os.Getenv("RUN_ADDRESS"); envServerRunAddress != "" {
		serverRunAddress = envServerRunAddress
	}

	if envDatabaseURI := os.Getenv("DATABASE_URI"); envDatabaseURI != "" {
		databaseURI = envDatabaseURI
	}

	if envAccrualSystemAddress := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); envAccrualSystemAddress != "" {
		accrualSystemAddress = envAccrualSystemAddress
	}

	return newServiceConfigBuilder().
		withServerRunAddress(serverRunAddress).
		withDatabaseURI(databaseURI).
		withAccrualSystemAddress(accrualSystemAddress).
		build(), nil
}
