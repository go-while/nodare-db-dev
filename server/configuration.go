package server

import (
	"errors"
	"fmt"
	//"github.com/go-while/nodare-db-dev/database"
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
	GetInt64(key string) int64
	GetUint32(key string) uint32
	GetUint64(key string) uint64
	GetBool(key string) bool
	IsSet(key string) bool
}

type ViperConfig struct {
	viper            *viper.Viper
	logs             ilog.ILOG
	mapsEnvsToConfig map[string]string
}

var (
	RTO int //	ReadTimeout:  time.Duration(RTO) * time.Second,
	WTO int //	WriteTimeout: time.Duration(WTO) * time.Second,
	ITO int // 	IdleTimeout:  time.Duration(ITO) * time.Second,
)

func (c *ViperConfig) checkFileExists(filePath string) bool {
	_, error := os.Stat(filePath)
	return !errors.Is(error, os.ErrNotExist)
} // end func checkFileExists

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
} // end func createDirectory

func (c *ViperConfig) createDefaultConfigFile(cfgFile string) {

	log.Printf("Creating default config")

	suadminuser := DEFAULT_SUPERADMIN
	suadminpass := utils.GenerateRandomString(DEFAULT_PW_LEN)

	c.viper.SetDefault(VK_ACCESS_SUPERADMIN_USER, suadminuser)
	c.viper.SetDefault(VK_ACCESS_SUPERADMIN_PASS, suadminpass)

	c.viper.SetDefault(VK_LOG_LOGLEVEL, DEFAULT_LOGLEVEL_STR)
	c.viper.SetDefault(VK_LOG_LOGFILE, DEFAULT_LOGS_FILE)

	log.Printf("createDefaultConfigFile: loglevel loaded to %s", c.viper.GetString(VK_LOG_LOGLEVEL))

	c.logs.SetLOGLEVEL(ilog.GetLOGLEVEL(c.viper.GetString(VK_LOG_LOGLEVEL)))

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
	c.viper.SetDefault(VK_SERVER_SOCKET_ACL, V_DEFAULT_SERVER_SOCKET_ACL)
	c.viper.SetDefault(VK_SERVER_WEB_ACL, V_DEFAULT_SERVER_WEB_ACL)

	log.Printf("WriteConfigAs %s", cfgFile)
	if c.logs.IfDebug() {
		c.PrintConfigsToConsole()
	}
	c.viper.WriteConfigAs(cfgFile)

	fmt.Printf("\n IMPORTANT!\n  Generated ADMIN credentials!\n     login: '%s'\n     password: '%s'\n\n ==> createDefaultConfigFile OK\n", suadminuser, suadminpass)

} // end func createDefaultConfigFile

func (c *ViperConfig) mapEnvsToConf() {

	c.mapsEnvsToConfig[VK_ACCESS_SUPERADMIN_USER] = "NDB_SUPERADMIN"
	c.mapsEnvsToConfig[VK_ACCESS_SUPERADMIN_PASS] = "NDB_SADMINPASS"

	c.mapsEnvsToConfig[VK_LOG_LOGLEVEL] = ENVK_LOGLEVEL
	c.mapsEnvsToConfig[VK_LOG_LOGFILE] = ENVK_LOGSFILE

	c.mapsEnvsToConfig[VK_SETTINGS_BASE_DIR] = ENVK_NDB_BASE_DIR
	c.mapsEnvsToConfig[VK_SETTINGS_DATA_DIR] = ENVK_NDB_DATA_DIR
	c.mapsEnvsToConfig[VK_SETTINGS_SETTINGS_DIR] = ENVK_NDB_CONFIG_DIR
	c.mapsEnvsToConfig[VK_SETTINGS_SUB_DICKS] = ENVK_NDB_SUB_DICKS

	c.mapsEnvsToConfig[VK_SEC_TLS_ENABLED] = ENVK_NDB_TLS_ENABLED
	c.mapsEnvsToConfig[VK_SEC_TLS_PRIVKEY] = ENVK_NDB_TLS_PRIVKEY
	c.mapsEnvsToConfig[VK_SEC_TLS_PUBCERT] = ENVK_NDB_TLS_PUBCERT

	c.mapsEnvsToConfig[VK_NET_WEBSRV_READ_TIMEOUT] = ENVK_NDB_WEBSRV_READ_TIMEOUT
	c.mapsEnvsToConfig[VK_NET_WEBSRV_WRITE_TIMEOUT] = ENVK_NDB_WEBSRV_WRITE_TIMEOUT
	c.mapsEnvsToConfig[VK_NET_WEBSRV_IDLE_TIMEOUT] = ENVK_NDB_WEBSRV_IDLE_TIMEOUT

	c.mapsEnvsToConfig[VK_SERVER_HOST] = ENVK_NDB_HOST
	c.mapsEnvsToConfig[VK_SERVER_PORT_TCP] = ENVK_NDB_PORT
	c.mapsEnvsToConfig[VK_SERVER_PORT_UDP] = ENVK_NDB_SERVER_UDP_PORT
	c.mapsEnvsToConfig[VK_SERVER_SOCKET_PATH] = ENVK_NDB_SERVER_SOCKET_PATH
	c.mapsEnvsToConfig[VK_SERVER_SOCKET_PORT_TCP] = ENVK_NDB_SERVER_SOCKET_TCP_PORT
	c.mapsEnvsToConfig[VK_SERVER_SOCKET_PORT_TLS] = ENVK_NDB_SERVER_SOCKET_TLS_PORT
	c.mapsEnvsToConfig[VK_SERVER_SOCKET_ACL] = ENVK_NDB_SERVER_SOCKET_ACL
	c.mapsEnvsToConfig[VK_SERVER_WEB_ACL] = ENVK_NDB_SERVER_WEB_ACL

}

func (c *ViperConfig) ReadConfigsFromEnvs() {
	c.logs.Debug("READ ENV VARS")
	for key, envK := range c.mapsEnvsToConfig {
		valueFromEnv, ok := os.LookupEnv(envK)
		if !ok {
			c.logs.Debug("NO ENV envK='%s'", envK)
			continue
		}
		c.logs.Info("NEW ENV envK='%s' ==> '%s'", envK, valueFromEnv)
		switch envK {
		case "LOGLEVEL":
			valueFromEnv = strings.ToUpper(valueFromEnv)
			c.logs.SetLOGLEVEL(ilog.GetLOGLEVEL(valueFromEnv))
		}
		c.viper.Set(key, valueFromEnv)
	}
} // end func ReadConfigsFromEnvs

func (c *ViperConfig) initDB() (sub_dicks int) {

	dbBaseDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error in getting current working directory: %v", err)
	}
	os.Setenv("NDB_BASE_DIR", dbBaseDir)

	c.createDirectory(filepath.Join(dbBaseDir, CONFIG_DIR))
	c.createDirectory(filepath.Join(dbBaseDir, c.viper.GetString(VK_SETTINGS_DATA_DIR)))

	RTO = c.viper.GetInt(VK_NET_WEBSRV_READ_TIMEOUT)
	WTO = c.viper.GetInt(VK_NET_WEBSRV_WRITE_TIMEOUT)
	ITO = c.viper.GetInt(VK_NET_WEBSRV_IDLE_TIMEOUT)
	sub_dicks = c.viper.GetInt(VK_SETTINGS_SUB_DICKS)
	return
} // end func initDB

func (c *ViperConfig) PrintConfigsToConsole() {
	fmt.Printf("Print all configs that were set\n")
	for key, _ := range c.mapsEnvsToConfig {
		fmt.Printf("Config value '%v': '%v'\n", key, c.viper.Get(key))
	}
}

func NewViperConf(cfgFile string, logs ilog.ILOG) (VConfig, int) {

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

	// reads config.toml
	if err := c.viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %v", err)
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			panic("Config file was not found")
		} else {
			panic("Config file was found, but another error was produced")
		}
	}

	// reading env results in overwriting config with env vars!
	c.ReadConfigsFromEnvs()

	sub_dicks := c.initDB()
	if c.logs.IfDebug() {
		c.PrintConfigsToConsole()
	}
	return c.viper, sub_dicks
} // end func NewViperConf

func (c *ViperConfig) Get(key string) interface{} {
	return c.viper.Get(key)
}

func (c *ViperConfig) GetString(key string) string {
	return c.viper.GetString(key)
}

func (c *ViperConfig) GetBool(key string) bool {
	return c.viper.GetBool(key)
}

func (c *ViperConfig) GetInt(key string) int {
	return c.viper.GetInt(key)
}

func (c *ViperConfig) GetInt64(key string) int64 {
	return c.viper.GetInt64(key)
}

func (c *ViperConfig) GetUint32(key string) uint32 {
	return c.viper.GetUint32(key)
}

func (c *ViperConfig) GetUint64(key string) uint64 {
	return c.viper.GetUint64(key)
}

func (c *ViperConfig) IsSet(key string) bool {
	return c.viper.IsSet(key)
}
