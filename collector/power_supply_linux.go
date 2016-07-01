// Copyright 2015 rektide
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build !nopowersupply

package collector

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const powerSupplySubsystem = "power_supply"
const powerSupplyNamespace = "power_supply"
var labelNames = []string{"chargeFullDesign", "model", "tech", "type", "serial", "voltageMinDesign"}

func MakeMap(strings []string) map[string]float64 {
	var m = make(map[string]float64)
	for i, val := range strings {
		m[val] = float64(i)
	}
	return m
}

var chargeTypeMap = MakeMap([]string{
	"Unknown",
	"N/A",
	"Trickle",
	"Fast",
})
func readChargeType (supply string) float64 {
	text, err := readFile(supply, "charge_type")
	if err != nil {
		return 0.0
	}
	val := chargeTypeMap[text]
	return val
}

var healthMap = MakeMap([]string{
	"Unknown",
	"Good",
	"Overheat",
	"Dead",
	"Over voltage",
	"Unspecified failure",
	"Cold",
	"Watchdog timer expire",
	"Safety timer expire",
})
func readHealth (supply string) float64 {
	text, err := readFile(supply, "health")
	if err != nil {
		return 0.0
	}
	val := healthMap[text]
	return val
}

var statusMap = MakeMap([]string{
	"Unknown",
	"Charging",
	"Discharging",
	"Not charging",
	"Full",
})
func readStatus (supply string) float64 {
	text, err := readFile(supply, "status")
	if err != nil {
		return 0.0
	}
	val := statusMap[text]
	return val
}



// Based on docs from https://www.kernel.org/doc/Documentation/power/power_supply_class.txt

var (
	alarmDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, powerSupplyNamespace, "alarm"),
		"Alarm state",
		labelNames, nil,
	)
	chargeFullDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, powerSupplyNamespace, "charge_full"),
		"Maximum charge in µAh.",
		labelNames, nil,
	)
	chargeNowDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, powerSupplyNamespace, "charge_now"),
		"Charge in µAh.",
		labelNames, nil,
	)
	chargeTypeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, powerSupplyNamespace, "charge_type"),
		"Charge category.",
		labelNames, nil,
	)
	currentNowDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, powerSupplyNamespace, "current_now"),
		"Current in µAh.",
		labelNames, nil,
	)
	cycleCountDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, powerSupplyNamespace, "cycle_count"),
		"Cycles on supply.",
		labelNames, nil,
	)
	healthDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, powerSupplyNamespace, "health"),
		"Cycles on supply.",
		labelNames, nil,
	)
	onlineDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, powerSupplyNamespace, "online"),
		"Device present and online.",
		labelNames, nil,
	)
	presentDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, powerSupplyNamespace, "present"),
		"Device present and online.",
		labelNames, nil,
	)
	statusDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, powerSupplyNamespace, "status"),
		"Status.",
		labelNames, nil,
	)
	typeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, powerSupplyNamespace, "type"),
		"Supply type",
		labelNames, nil,
	)
	voltageNowDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, powerSupplyNamespace, "voltage_now"),
		"Supply voltage.",
		labelNames, nil,
	)
)

var (
	ignoredDevices = flag.String("collector.power_supply.ignored-devices", "^(BAT|AC)\\d+$", "Regexp of devices to ignore for power_supply.")
)

type powerSupplyCollector struct {
	ignoredDevicesPattern *regexp.Regexp
}

func init() {
	Factories[powerSupplySubsystem] = NewPowerSupplyCollector
}

// Takes a prometheus registry and returns a new Collector exposing
// power_supply system stats.
func NewPowerSupplyCollector() (Collector, error) {
	pattern := regexp.MustCompile(*ignoredDevices)
	return &powerSupplyCollector{
		ignoredDevicesPattern: pattern,
	}, nil
}

func readFile(supplyPath string, attribute string) (string, error) {
	file, err := os.Open(path.Join(supplyPath, attribute))
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Scan()
	text := scanner.Text()
	return text, nil
}

func readFloat(supplyPath string, attribute string) (float64, error) {
	text, err := readFile(supplyPath, attribute)
	if err != nil {
		return 0, err
	}
	num, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, err
	}
	return num, nil
}

func (c *powerSupplyCollector) Update(ch chan<- prometheus.Metric) (err error) {
	supplies, err := filepath.Glob(sysFilePath("class/power_supply/*[0-9]*"))
	if err != nil {
		return fmt.Errorf("couldn't get powerSupply: %s", err)
	}

	for _, supply := range supplies {
		if c.ignoredDevicesPattern.MatchString(supply) {
			log.Debugf("Ignoring device: %s", supply)
			continue
		}
		chargeFullDesign, _ := readFile(supply, "charge_full_design")
		model, _ := readFile(supply, "model_name")
		tech, _ := readFile(supply, "technology")
		type_, _ := readFile(supply, "type")
		serial_number, _ := readFile(supply, "serial_number")
		voltage_min_design, _ := readFile(supply, "voltage_min_design")

		alarm, _ := readFloat(supply, "alarm")
		ch <- prometheus.MustNewConstMetric(
			alarmDesc, prometheus.GaugeValue, alarm,
			chargeFullDesign, model, tech, type_, serial_number, voltage_min_design)

		chargeFull, _ := readFloat(supply, "charge_full")
		ch <- prometheus.MustNewConstMetric(
			chargeFullDesc, prometheus.GaugeValue, chargeFull,
			chargeFullDesign, model, tech, type_, serial_number, voltage_min_design)

		chargeType, _ := readFloat(supply, "charge_type")
		ch <- prometheus.MustNewConstMetric(
			chargeTypeDesc, prometheus.GaugeValue, chargeType,
			chargeFullDesign, model, tech, type_, serial_number, voltage_min_design)

		chargeNow, _ := readFloat(supply, "charge_now")
		ch <- prometheus.MustNewConstMetric(
			chargeNowDesc, prometheus.GaugeValue, chargeNow,
			chargeFullDesign, model, tech, type_, serial_number, voltage_min_design)

		currentNow, _ := readFloat(supply, "current_now")
		ch <- prometheus.MustNewConstMetric(
			currentNowDesc, prometheus.GaugeValue, currentNow,
			chargeFullDesign, model, tech, type_, serial_number, voltage_min_design)

		cycleCount, _ := readFloat(supply, "cycle_count")
		ch <- prometheus.MustNewConstMetric(
			cycleCountDesc, prometheus.GaugeValue, cycleCount,
			chargeFullDesign, model, tech, type_, serial_number, voltage_min_design)

		health := readHealth(supply)
		ch <- prometheus.MustNewConstMetric(
			healthDesc, prometheus.GaugeValue, health,
			chargeFullDesign, model, tech, type_, serial_number, voltage_min_design)

		online, _ := readFloat(supply, "online")
		ch <- prometheus.MustNewConstMetric(
			onlineDesc, prometheus.GaugeValue, online,
			chargeFullDesign, model, tech, type_, serial_number, voltage_min_design)

		present, _ := readFloat(supply, "present")
		ch <- prometheus.MustNewConstMetric(
			presentDesc, prometheus.GaugeValue, present,
			chargeFullDesign, model, tech, type_, serial_number, voltage_min_design)

		status := readStatus(supply)
		ch <- prometheus.MustNewConstMetric(
			statusDesc, prometheus.GaugeValue, status,
			chargeFullDesign, model, tech, type_, serial_number, voltage_min_design)

		voltageNow, _ := readFloat(supply, "voltage_now")
		ch <- prometheus.MustNewConstMetric(
			voltageNowDesc, prometheus.GaugeValue, voltageNow,
			chargeFullDesign, model, tech, type_, serial_number, voltage_min_design)
	}
	return err
}
