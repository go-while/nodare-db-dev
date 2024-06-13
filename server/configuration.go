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

type VConfig interface {
	Get(key string) interface{}
	GetString(key string) string
	GetInt(key string) int
	GetUint32(key string) uint32
	GetUint64(key string) uint64
	GetBool(key string) bool
	IsSet(key string) bool
}

type ViperConfig struct {
	viper            *viper.Viper
	logs           ilog.ILOG
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
		c.logs.Info("Directory created successfully: %s", dirPath)
	case os.IsExist(err):
		c.logs.Info("Directory already exists: %s", dirPath)
	default:
		c.logs.Info("Error creating directory: %v", err)
	}
}

func (c *ViperConfig) createDefaultConfigFile(cfgFile string) {

	log.Printf("Creating default config")

	suadminuser := DEFAULT_SUPERADMIN
	suadminpass := utils.GenerateRandomString(DEFAULT_PW_LEN)

	c.viper.SetDefault(VK_ACCESS_SUPERADMIN_USER, suadminuser)
	c.viper.SetDefault(VK_ACCESS_SUPERADMIN_PASS, suadminpass)

	c.viper.SetDefault(VK_LOG_LOGLEVEL, DEFAULT_LOGLEVEL_STR)
	c.viper.SetDefault(VK_LOG_LOGFILE, DEFAULT_LOGS_FILE)

	c.viper.SetDefault(VK_SETTINGS_BASE_DIR, DOT)
	c.viper.SetDefault(VK_SETTINGS_DATA_DIR, DATA_DIR)
	c.viper.SetDefault(VK_SETTINGS_SETTINGS_DIR, CONFIG_DIR)
	c.viper.SetDefault(VK_SETTINGS_SUB_DICKS, V_DEFAULT_SUB_DICKS)

	c.viper.SetDefault(VK_SEC_TLS_ENABLED, V_DEFAULT_TLS_ENABLED)
	// /etc/letsencrypt/live/(sub.)domain.com/fullchain.pem
	c.viper.SetDefault(VK_SEC_TLS_PRIVKEY, filepath.Join(CONFIG_DIR, DEFAULT_TLS_PUBCERT))
	// /etc/letsencrypt/live/(sub.)domain.com/privkey.pem
	c.viper.SetDefault(VK_SEC_TLS_PUBCERT, filepath.Join(CONFIG_DIR, DEFAULT_TLS_PRIVKEY))

	c.viper.SetDefault(VK_NET_WEBSRV_READ_TIMEOUT, V_DEFAULT_NET_WEBSRV_READ_TIMEOUT)
	c.viper.SetDefault(VK_NET_WEBSRV_WRITE_TIMEOUT, V_DEFAULT_NET_WEBSRV_WRITE_TIMEOUT)
	c.viper.SetDefault(VK_NET_WEBSRV_IDLE_TIMEOUT, V_DEFAULT_NET_WEBSRV_IDLE_TIMEOUT)

	c.viper.SetDefault(VK_SERVER_HOST, DEFAULT_SERVER_ADDR)
	c.viper.SetDefault(VK_SERVER_PORT_TCP, DEFAULT_SERVER_TCP_PORT)
	c.viper.SetDefault(VK_SERVER_PORT_UDP, DEFAULT_SERVER_UDP_PORT)
	c.viper.SetDefault(VK_SERVER_SOCKET_PATH, DEFAULT_SERVER_SOCKET_PATH)
	c.viper.SetDefault(VK_SERVER_SOCKET_PORT_TCP, DEFAULT_SERVER_SOCKET_TCP_PORT)
	c.viper.SetDefault(VK_SERVER_SOCKET_PORT_TLS, DEFAULT_SERVER_SOCKET_TLS_PORT)

	c.viper.WriteConfigAs(cfgFile)

	fmt.Printf("\nIMPORTANT! Generated ADMIN credentials! \n superadmin login: '%s' password: %s\n\n", suadminuser, suadminpass)
}

func (c *ViperConfig) mapEnvsToConf() {

	c.mapsEnvsToConfig[VK_ACCESS_SUPERADMIN_USER] = "NDB_SUPERADMIN"
	c.mapsEnvsToConfig[VK_ACCESS_SUPERADMIN_PASS] = "NDB_SADMINPASS"

	c.mapsEnvsToConfig[VK_LOG_LOGLEVEL] = "LOGLEVEL"
	c.mapsEnvsToConfig[VK_LOG_LOGFILE] = "LOGS_FILE"

	c.mapsEnvsToConfig[VK_SETTINGS_BASE_DIR] = "NDB_BASE_DIR"
	c.mapsEnvsToConfig[VK_SETTINGS_DATA_DIR] = "NDB_DATA_DIR"
	c.mapsEnvsToConfig[VK_SETTINGS_SETTINGS_DIR] = "NDB_CONFIG_DIR"
	c.mapsEnvsToConfig[VK_SETTINGS_SUB_DICKS] = "NDB_SUB_DICKS"

	c.mapsEnvsToConfig[VK_SEC_TLS_ENABLED] = "NDB_TLS_ENABLED"
	c.mapsEnvsToConfig[VK_SEC_TLS_PRIVKEY] = "NDB_TLS_KEY"
	c.mapsEnvsToConfig[VK_SEC_TLS_PUBCERT] = "NDB_TLS_CRT"

	c.mapsEnvsToConfig[VK_NET_WEBSRV_READ_TIMEOUT] = "NDB_WEBSRV_READ_TIMEOUT"
	c.mapsEnvsToConfig[VK_NET_WEBSRV_WRITE_TIMEOUT] = "NDB_WEBSRV_WRITE_TIMEOUT"
	c.mapsEnvsToConfig[VK_NET_WEBSRV_IDLE_TIMEOUT] = "NDB_WEBSRV_IDLE_TIMEOUT"

	c.mapsEnvsToConfig[VK_SERVER_HOST] = "NDB_HOST"
	c.mapsEnvsToConfig[VK_SERVER_PORT_TCP] = "NDB_PORT"
	c.mapsEnvsToConfig[VK_SERVER_PORT_UDP] = "DEFAULT_SERVER_UDP_PORT"
	c.mapsEnvsToConfig[VK_SERVER_SOCKET_PATH] = "DEFAULT_SERVER_SOCKET_PATH"
	c.mapsEnvsToConfig[VK_SERVER_SOCKET_PORT_TCP] = "DEFAULT_SERVER_SOCKET_TCP_PORT"
	c.mapsEnvsToConfig[VK_SERVER_SOCKET_PORT_TLS] = "DEFAULT_SERVER_SOCKET_TLS_PORT"

}

func (c *ViperConfig) ReadConfigsFromEnvs() {
	c.logs.Debug("READ ENV VARS")
	for key, value := range c.mapsEnvsToConfig {
		valueFromEnv, ok := os.LookupEnv(value)
		if ok {
			c.logs.Info("NEW ENV VK='%s' envK='%v' valueFromEnv='%s", key, value, valueFromEnv)
			c.viper.Set(key, valueFromEnv)
		} else {
			c.logs.Debug("NO ENV VK='%s' valueFromEnv='%s' !ok", key, valueFromEnv)
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
	c.createDirectory(filepath.Join(dbBaseDir, c.viper.GetString(VK_SETTINGS_DATA_DIR)))

	setSUBDICKS := c.viper.GetUint32(VK_SETTINGS_SUB_DICKS)
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

func NewViperConf(cfgFile string, logs ilog.ILOG) (VConfig, uint32) {

	if len(strings.TrimSpace(cfgFile)) == 0 {
		log.Printf("No config file in '%s' was supplied. Using default value: %s", cfgFile, DEFAULT_CONFIG_FILE)
		cfgFile = DEFAULT_CONFIG_FILE
	}

	c := &ViperConfig{viper: viper.New(), logs: logs, mapsEnvsToConfig: make(map[string]string, 32)}
	c.viper.SetConfigType("toml")
	c.mapEnvsToConf()

	if !c.checkFileExists(cfgFile) {
		log.Printf("Configuration file does not exist: %s", cfgFile)
		c.createDefaultConfigFile(cfgFile)
	}

	c.logs.Info("Using config file: %s", cfgFile)

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
} // end func NewViperConf
