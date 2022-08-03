package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/i4de/rulex/core"
	"github.com/i4de/rulex/engine"
	"github.com/i4de/rulex/glogger"
	httpserver "github.com/i4de/rulex/plugin/http_server"

	"github.com/i4de/rulex/typex"
)

/*
*
* Test 485 sensor gateway
*
 */
func main() {
	mainConfig := core.InitGlobalConfig("rulex.ini")
	glogger.StartGLogger(true, core.GlobalConfig.LogPath)
	glogger.StartLuaLogger(core.GlobalConfig.LuaLogPath)
	core.StartStore(core.GlobalConfig.MaxQueueSize)
	core.SetLogLevel()
	core.SetPerformance()
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGABRT, syscall.SIGTERM)
	engine := engine.NewRuleEngine(mainConfig)
	engine.Start()

	hh := httpserver.NewHttpApiServer()

	// HttpApiServer loaded default
	if err := engine.LoadPlugin("plugin.http_server", hh); err != nil {
		glogger.GLogger.Error(err)
	}
	// RTU485_THER Inend
	RTU485Device := typex.NewDevice("RTU485_THER",
		"温湿度采集器", "温湿度采集器", "", map[string]interface{}{
			"slaverIds": []uint8{1, 2},
			"timeout":   5,
			"frequency": 5,
			"config": map[string]interface{}{
				"uart":     "COM4",
				"dataBits": 8,
				"parity":   "N",
				"stopBits": 1,
				"baudRate": 4800,
			},
			"registers": []map[string]interface{}{
				{
					"tag":      "node1",
					"function": 3,
					"slaverId": 1,
					"address":  0,
					"quantity": 2,
				},
				{
					"tag":      "node2",
					"function": 3,
					"slaverId": 2,
					"address":  0,
					"quantity": 2,
				},
			},
		})
	RTU485Device.UUID = "RTU485Device1"
	if err := engine.LoadDevice(RTU485Device); err != nil {
		glogger.GLogger.Error(err)
	}
	mqttOutEnd := typex.NewOutEnd(
		"MQTT",
		"MQTT桥接",
		"MQTT桥接", map[string]interface{}{
			"Host":     "42.193.180.26",
			"Port":     1883,
			"ClientId": "PLAT00000001",
			"Username": "PLAT00000001",
			"Password": "PLAT00000001",
			"PubTopic": "$thing/up/property/PLAT/PLAT00000001",
			"SubTopic": "$thing/down/property/PLAT/PLAT00000001",
		},
	)
	mqttOutEnd.UUID = "mqttOutEnd-iothub"
	if err := engine.LoadOutEnd(mqttOutEnd); err != nil {
		glogger.GLogger.Error(err)
	}
	rule := typex.NewRule(engine,
		"uuid",
		"数据推送至IOTHUB",
		"数据推送至IOTHUB",
		[]string{},
		[]string{RTU485Device.UUID}, // 数据来自网关设备,所以这里需要配置设备ID
		`function Success() end`,
		`
Actions = {function(data)
	for tag, v in pairs(rulexlib:J2T(data)) do
		local ts = rulexlib:TsUnixNano()
		local value = rulexlib:J2T(v['value'])
		value['tag']= tag;
		local jsont = {
			method = 'report',
			requestId = ts,
			timestamp = ts,
			params = value
		}
		-- print('mqttOutEnd-iothub', rulexlib:T2J(jsont))
		rulexlib:DataToMqtt('mqttOutEnd-iothub', rulexlib:T2J(jsont))
	end
	return true, data
end}
`,
		`function Failed(error) end`)
	if err := engine.LoadRule(rule); err != nil {
		glogger.GLogger.Error(err)
	}
	s := <-c
	glogger.GLogger.Warn("Received stop signal:", s)
	engine.Stop()
	os.Exit(0)
}
