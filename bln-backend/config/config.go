package config

import (
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/viper"
)

type Config struct {
	Env      string `mapstructure:"ENV"`
	Port     string `mapstructure:"PORT"`
	Database struct {
		Host      string `mapstructure:"HOST"`
		Port      string `mapstructure:"PORT"`
		User      string `mapstructure:"USER"`
		Password  string `mapstructure:"PASSWORD"`
		Name      string `mapstructure:"NAME"`
		IsMigrate bool   `mapstructure:"ISMIGRATE"`
	} `mapstructure:"DATABASE"`
	JWT struct {
		Secret     string `mapstructure:"SECRET"`
		ExpireTime int    `mapstructure:"EXPIRE_TIME"`
		Issuer     string `mapstructure:"ISSUER"`
		Subject    string `mapstructure:"SUBJECT"`
		Audience   string `mapstructure:"AUDIENCE"`
		Algorithm  string `mapstructure:"ALGORITHM"`
	} `mapstructure:"JWT"`
	IpfsPinana struct {
		UploadUrl string `mapstructure:"UPLOAD_URL"`
		ViewUrl   string `mapstructure:"VIEW_URL"`
		ApiKey    string `mapstructure:"API_KEY"`
		ApiSecret string `mapstructure:"API_SECRET"`
		Jwt       string `mapstructure:"JWT"`
	} `mapstructure:"IPFS_PINATA"`
	NetWork struct {
		RpcUrl            string `mapstructure:"RPC_URL"`
		AccountPrivateKey string `mapstructure:"ACCOUNT_PRIVATEKEY"`
	} `mapstructure:"NETWORK"`
	Listener struct {
		Enabled         bool   `mapstructure:"ENABLED"`
		StartBlock      uint64 `mapstructure:"START_BLOCK"`
		PollIntervalSec int    `mapstructure:"POLL_INTERVAL_SEC"`
		BatchSize       uint64 `mapstructure:"BATCH_SIZE"`
	} `mapstructure:"LISTENER"`
}

var AppConfig Config

// LoadConfig 加载配置文件（支持按 GO_ENV 切换）
func LoadConfig() {
	env := os.Getenv("GO_ENV")
	if env == "" {
		env = "development"
	}

	// 根据环境选择配置文件
	if env == "production" {
		viper.SetConfigName("config.production")
	} else {
		viper.SetConfigName("config")
	}

	viper.SetConfigType("yml") // 配置文件类型

	configPath := findRoot() + "/config"
	viper.AddConfigPath(configPath) // 添加配置文件路径

	if err := viper.ReadInConfig(); err != nil {
		log.Errorf("Error reading config file, %s", err)
		return
	}

	if err := viper.Unmarshal(&AppConfig); err != nil {
		log.Errorf("Unable to decode into struct, %v", err)
		return
	}

	AppConfig.Env = env
	log.Infof("config loaded for env=%s, file=%s", env, viper.ConfigFileUsed())
}

func findRoot() string {
	if os.Getenv("GO_ENV") == "production" {
		exePath, err := os.Executable()
		if err != nil {
			panic(err)
		}
		return filepath.Dir(exePath)
	} else {
		dir, err := os.Getwd()
		if err != nil {
			panic(err)
		}

		// 不断向上查找 go.mod 文件
		for {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				return dir // 找到 go.mod 文件，返回当前目录
			}

			// 向上一级目录
			parent := filepath.Dir(dir)
			if parent == dir {
				// 已经到根目录，停止
				break
			}
			dir = parent
		}

		panic("could not find go.mod")
	}

}
