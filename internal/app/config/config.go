package config

import (
	"flag"
	"os"
)

type ServerConfig struct {
	ServerRunAddress     string
	DatabaseURI          string
	AccrualSystemAddress string
	JWTSecretKey         string
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

func (sc *serverConfigBuilder) withJWTSecretKey(JWTSecretKey string) *serverConfigBuilder {
	sc.serviceConfig.JWTSecretKey = JWTSecretKey
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
		jwtSecretKey         string
	)

	flag.StringVar(&serverRunAddress, "a", "localhost:8080", "address:port to run server")
	flag.StringVar(&databaseURI, "d", "", "connection string for driver to establish connection to he DB")
	flag.StringVar(&accrualSystemAddress, "r", "", "address of the accrual calculation system")
	flag.StringVar(&jwtSecretKey, "j", "temp_secret_key", "jwt secret key")
	flag.Parse()

	if envServerRunAddress, ok := os.LookupEnv("RUN_ADDRESS"); envServerRunAddress != "" && ok {
		serverRunAddress = envServerRunAddress
	}

	if envDatabaseURI, ok := os.LookupEnv("DATABASE_URI"); envDatabaseURI != "" && ok {
		databaseURI = envDatabaseURI
	}

	if envAccrualSystemAddress, ok := os.LookupEnv("ACCRUAL_SYSTEM_ADDRESS"); envAccrualSystemAddress != "" && ok {
		accrualSystemAddress = envAccrualSystemAddress
	}

	if envJWTSecretKey, ok := os.LookupEnv("JWT_SECRET_KEY"); envJWTSecretKey != "" && ok {
		jwtSecretKey = envJWTSecretKey
	}

	return newServiceConfigBuilder().
		withServerRunAddress(serverRunAddress).
		withDatabaseURI(databaseURI).
		withAccrualSystemAddress(accrualSystemAddress).
		withJWTSecretKey(jwtSecretKey).
		build(), nil
}
