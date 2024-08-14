package main

import (
	"github.com/common-nighthawk/go-figure"
	"github.com/spf13/viper"
	"log"
	"os"
	"strconv"
	"strings"
)

func main() {
	art := figure.NewColorFigure("Jerusalem", "slant", "green", true)
	art.Print()

	print("\n\n")

	viper.AutomaticEnv()
	viper.SetConfigName("server")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.config/jerusalem")
	viper.AddConfigPath("/etc/jerusalem/")

	if len(os.Args) > 1 && strings.HasPrefix(os.Args[1], "--config") {
		configPath := strings.Split(os.Args[1], "=")
		if len(configPath) > 1 {
			viper.SetConfigFile(configPath[1])
		}
	}

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Config file not found: %v", err)
	}

	// Read variables
	pStr := strings.TrimSpace(viper.GetString("server_port"))
	s := strings.TrimSpace(viper.GetString("secret_key"))
	pr := strings.TrimSpace(viper.GetString("port_range"))

	if pr == "" || !strings.Contains(pr, "...") {
		log.Fatal("missing or invalid PORT_RANGE")
	}
	if pStr == "" {
		log.Fatal("missing SERVER_PORT")
	}
	if s == "" {
		s = DefaultSecret
	}
	if len(s) != 64 {
		log.Fatal("SECRET_KEY must be 64 characters long")
	}

	prParts := strings.Split(pr, "...")
	minP, err := strconv.ParseUint(prParts[0], 10, 16)
	if err != nil || minP < 1024 || minP > 65535 {
		log.Fatalf("invalid PORT_RANGE: %v", err)
	}
	maxP, err := strconv.ParseUint(prParts[1], 10, 16)
	if err != nil || maxP < 1024 || maxP > 65535 || maxP < minP {
		log.Fatalf("invalid PORT_RANGE: %v", err)
	}
	p, err := strconv.ParseUint(pStr, 10, 16)
	if err != nil || p < 1024 || p > 65535 {
		log.Fatalf("invalid SERVER_PORT: %s", pStr)
	}
	db, err := NewTCPClientRepository()
	if err != nil {
		log.Fatalf("error creating database: %v", err)
	}

	svr := NewServer(uint16(p), RangeInclusive{min: uint16(minP), max: uint16(maxP)}, db, &s)
	if err := svr.StartServer(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
