package server

import (
	"errors"
	"fmt"

	"github.com/go-while/nodare-db-dev/logger"
	"github.com/go-while/nodare-db-dev/utils"
	"github.com/spf13/viper"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var AVAIL_SUBDICKS = []uint32{10, 100, 1000, 10000, 100000, 1000000}

type VConfig interface {
	Get(key string) interface{}
	GetString(key string) string
	GetInt(key string) int
	GetUint32(key string) uint32
	GetBool(key string) bool
	IsSet(key string) bool
}

type ViperConfig struct {
	//*viper.Viper
	viper            *viper.Viper
	logger           ilog.ILOG
	mapsEnvsToConfig map[string]string
}

//var mapsEnvsToConfig map[string]string = make(map[string]string)

func (c *ViperConfig) checkFileExists(filePath string) bool {
	_, error := os.Stat(filePath)
	return !errors.Is(error, os.ErrNotExist)
}

func (c *ViperConfig) createDirectory(dirPath string) {
	err := os.MkdirAll(dirPath, 0755)
	switch {
	case err == nil:
		c.logger.Info("Directory created successfully: %s", dirPath)
	case os.IsExist(err):
		c.logger.Info("Directory already exists: %s", dirPath)
	default:
		c.logger.Info("Error creating directory: %v", err)
	}
}

func (c *ViperConfig) createDefaultConfigFile(cfgFile string) {

	log.Printf("Creating default config file")
	suadminuser := DEFAULT_ADMIN
	suadminpass := utils.GenerateRandomString(DEFAULT_PW_LEN)

	c.viper.SetDefault("server.superadmin_user", suadminuser)
	c.viper.SetDefault("server.superadmin_pass", suadminpass)

	c.viper.SetDefault("log.log_level", DEFAULT_LOGLEVEL_STR)
	c.viper.SetDefault("log.log_file", DEFAULT_LOGS_FILE)

	c.viper.SetDefault("settings.base_dir", ".")
	c.viper.SetDefault("settings.data_dir", DATA_DIR)
	c.viper.SetDefault("settings.settings_dir", CONFIG_DIR)
	c.viper.SetDefault("settings.sub_dicks", "1000")

	c.viper.SetDefault("security.tls_enabled", false)
	// /etc/letsencrypt/live/(sub.)domain.com/fullchain.pem
	c.viper.SetDefault("security.tls_cert_private", filepath.Join(CONFIG_DIR, DEFAULT_TLS_PUBCERT))
	// /etc/letsencrypt/live/(sub.)domain.com/privkey.pem
	c.viper.SetDefault("security.tls_cert_public", filepath.Join(CONFIG_DIR, DEFAULT_TLS_PRIVKEY))

	c.viper.SetDefault("network.websrv_read_timeout", 5)
	c.viper.SetDefault("network.websrv_write_timeout", 10)
	c.viper.SetDefault("network.websrv_idle_timeout", 120)

	c.viper.SetDefault("server.host", DEFAULT_SERVER_ADDR)
	c.viper.SetDefault("server.port", DEFAULT_SERVER_TCP_PORT)
	c.viper.SetDefault("server.port_udp", DEFAULT_SERVER_UDP_PORT)
	c.viper.SetDefault("server.socket_path", DEFAULT_SERVER_SOCKET_PATH)
	c.viper.SetDefault("server.socket_tcpport", DEFAULT_SERVER_SOCKET_TCP_PORT)
	c.viper.SetDefault("server.socket_tlsport", DEFAULT_SERVER_SOCKET_TLS_PORT)

	c.viper.WriteConfigAs(cfgFile)

	fmt.Printf("\nIMPORTANT! Generated ADMIN credentials! \n superadmin login: '%s' password: %s\n\n", suadminuser, suadminpass)
}

func (c *ViperConfig) mapEnvsToConf() {

	c.mapsEnvsToConfig["server.superadmin_user"] = "NDB_SUPERADMIN"
	c.mapsEnvsToConfig["server.superadmin_pass"] = "NDB_SADMINPASS"

	c.mapsEnvsToConfig["log.log_level"] = "LOGLEVEL"
	c.mapsEnvsToConfig["log.log_file"] = "LOGS_FILE"

	c.mapsEnvsToConfig["settings.base_dir"] = "NDB_BASE_DIR"
	c.mapsEnvsToConfig["settings.data_dir"] = "NDB_DATA_DIR"
	c.mapsEnvsToConfig["settings.settings_dir"] = "NDB_CONFIG_DIR"
	c.mapsEnvsToConfig["settings.sub_dicks"] = "NDB_SUB_DICKS"

	c.mapsEnvsToConfig["security.tls_enabled"] = "NDB_TLS_ENABLED"
	c.mapsEnvsToConfig["security.tls_cert_private"] = "NDB_TLS_KEY"
	c.mapsEnvsToConfig["security.tls_cert_public"] = "NDB_TLS_CRT"

	c.mapsEnvsToConfig["network.websrv_read_timeout"] = "NDB_WEBSRV_READ_TIMEOUT"
	c.mapsEnvsToConfig["network.websrv_write_timeout"] = "NDB_WEBSRV_WRITE_TIMEOUT"
	c.mapsEnvsToConfig["network.websrv_idle_timeout"] = "NDB_WEBSRV_IDLE_TIMEOUT"

	c.mapsEnvsToConfig["server.host"] = "NDB_HOST"
	c.mapsEnvsToConfig["server.port"] = "NDB_PORT"
	c.mapsEnvsToConfig["server.port_udp"] = "DEFAULT_SERVER_UDP_PORT"
	c.mapsEnvsToConfig["server.socket_path"] = "DEFAULT_SERVER_SOCKET_PATH"
	c.mapsEnvsToConfig["server.socket_tcpport"] = "DEFAULT_SERVER_SOCKET_TCP_PORT"
	c.mapsEnvsToConfig["server.socket_tlsport"] = "DEFAULT_SERVER_SOCKET_TLS_PORT"

}

func (c *ViperConfig) ReadConfigsFromEnvs() {
	log.Printf("READ ENV VARS")
	for key, value := range c.mapsEnvsToConfig {
		valueFromEnv, ok := os.LookupEnv(value)
		if ok {
			log.Printf("GOT NEW ENV key='%s' v='%v' valueFromEnv='%s", key, value, valueFromEnv)
			c.viper.Set(key, valueFromEnv)
		} else {
			log.Printf("NO ENV: key='%s' val='%s' !ok", key, valueFromEnv)
		}
	}
}

func (c *ViperConfig) initDB() (sub_dicks uint32) {

	dbBaseDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error in getting current working directory: %v", err)
	}
	os.Setenv("NDB_BASE_DIR", dbBaseDir)

	c.createDirectory(filepath.Join(dbBaseDir, CONFIG_DIR))
	c.createDirectory(filepath.Join(dbBaseDir, c.viper.GetString("settings.data_dir")))

	setSUBDICKS := c.viper.GetUint32("settings.sub_dicks")
	for _, v := range AVAIL_SUBDICKS {
		if setSUBDICKS == v {
			sub_dicks = setSUBDICKS
			return
		}
	}
	// reached here we did not get a valid sub_dicks value from config
	// always return at least 10 so we don't fail
	log.Printf("WARN invalid sub_dicks value=%d !! defaulted to 1000", setSUBDICKS)
	return 1000
} // end func initDB

func (c *ViperConfig) PrintConfigsToConsole() {
	fmt.Printf("Print all configs that were set\n")
	for key, _ := range c.mapsEnvsToConfig {
		fmt.Printf("Config value '%v': '%v'\n", key, c.viper.Get(key))
	}
}

func NewConfiguration(cfgFile string, logger ilog.ILOG) (VConfig, uint32) {

	//v := viper.New()
	//v.mapEnvsToConf()
	//v.SetConfigType("toml")

	if len(strings.TrimSpace(cfgFile)) == 0 {
		log.Printf("No config file in '%s' was supplied. Using default value: %s", cfgFile, DEFAULT_CONFIG_FILE)
		cfgFile = DEFAULT_CONFIG_FILE
	}

	c := &ViperConfig{viper: viper.New(), logger: logger, mapsEnvsToConfig: make(map[string]string, 32)}
	c.viper.SetConfigType("toml")
	c.mapEnvsToConf()

	if !c.checkFileExists(cfgFile) {
		log.Printf("Configuration file does not exist: %s", cfgFile)
		c.createDefaultConfigFile(cfgFile)
	}

	log.Printf("Using config file: %s", cfgFile)

	c.viper.SetConfigFile(cfgFile)

	if err := c.viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %v", err)
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			panic("Config file was not found")
		} else {
			panic("Config file was found, but another error was produced")
		}
	}

	c.ReadConfigsFromEnvs()
	sub_dicks := c.initDB()
	c.PrintConfigsToConsole()
	return c.viper, sub_dicks
}

/*
func (c *ViperConfig) Get(key string) interface{} {
	return c.viper.Get(key)
}

func (c *ViperConfig) GetString(key string) string {
	return c.viper.GetString(key)
}

func (c *ViperConfig) GetBool(key string) bool {
	return c.viper.GetBool(key)
}

func (c *ViperConfig) IsSet(key string) bool {
	return c.viper.IsSet(key)
}
*/
