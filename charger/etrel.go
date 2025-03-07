package charger

// LICENSE

// Copyright (c) 2019-2022 andig

// This module is NOT covered by the MIT license. All rights reserved.

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/modbus"
	"github.com/volkszaehler/mbmd/encoding"
)

// https://github.com/RustyDust/sonnen-charger/blob/main/Etrel%20INCH%20SmartHome%20Modbus%20TCPRegisters.xlsx

const (
	// read
	etrelRegChargeStatus = 0
	etrelRegCurrents     = 14 // 16, 18
	etrelRegPower        = 26
	etrelRegSerial       = 990
	etrelRegModel        = 1000
	etrelRegHWVersion    = 1010
	etrelRegSWVersion    = 1015
	// etrelRegMaxPhaseCurrent  = 2
	// etrelRegTargetCurrent    = 4 // power mgmt or modbus
	// etrelRegSessionEnergy = 30
	// etrelRegChargeTime       = 32
	// etrelRegCustomMaxCurrent = 1028

	// write
	etrelRegMaxCurrent = 8
	// etrelRegStop       = 1
	// etrelRegPause      = 2
	// etrelRegMaxPower   = 11
)

// Etrel is an api.Charger implementation for Etrel/Sonnen wallboxes
type Etrel struct {
	log     *util.Logger
	conn    *modbus.Connection
	current float32
}

func init() {
	registry.Add("etrel", NewEtrelFromConfig)
}

// NewEtrelFromConfig creates a Etrel charger from generic config
func NewEtrelFromConfig(other map[string]interface{}) (api.Charger, error) {
	cc := modbus.TcpSettings{
		ID: 1,
	}

	if err := util.DecodeOther(other, &cc); err != nil {
		return nil, err
	}

	return NewEtrel(cc.URI, cc.ID)
}

// NewEtrel creates a Etrel charger
func NewEtrel(uri string, id uint8) (*Etrel, error) {
	conn, err := modbus.NewConnection(uri, "", "", 0, modbus.Tcp, id)
	if err != nil {
		return nil, err
	}

	// if !sponsor.IsAuthorized() {
	// 	return nil, api.ErrSponsorRequired
	// }

	log := util.NewLogger("etrel")
	conn.Logger(log.TRACE)

	wb := &Etrel{
		log:     log,
		conn:    conn,
		current: 6,
	}

	return wb, nil
}

// Status implements the api.Charger interface
func (wb *Etrel) Status() (api.ChargeStatus, error) {
	b, err := wb.conn.ReadInputRegisters(etrelRegChargeStatus, 1)
	if err != nil {
		return api.StatusNone, err
	}

	// 0 Unknown
	// 1 SocketAvailable
	// 2 WaitingForVehicleToBeConnected
	// 3 WaitingForVehicleToStart
	// 4 Charging
	// 5 ChargingPausedByEv
	// 6 ChargingPausedByEvse
	// 7 ChargingEnded
	// 8 ChargingFault
	// 9 UnpausingCharging
	// 10 Unavailable

	switch u := binary.BigEndian.Uint16(b); u {
	case 1, 2:
		return api.StatusA, nil
	case 3, 5, 6, 7, 9:
		return api.StatusB, nil
	case 4:
		return api.StatusC, nil
	default:
		return api.StatusNone, fmt.Errorf("invalid status: %d", u)
	}
}

// Enabled implements the api.Charger interface
func (wb *Etrel) Enabled() (bool, error) {
	b, err := wb.conn.ReadHoldingRegisters(etrelRegMaxCurrent, 2)
	if err != nil {
		return false, err
	}

	return encoding.Float32(b) > 0, nil
}

// Enable implements the api.Charger interface
func (wb *Etrel) Enable(enable bool) error {
	var current float32
	if enable {
		current = wb.current
	}

	return wb.setCurrent(current)
}

func (wb *Etrel) setCurrent(current float32) error {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, math.Float32bits(current))

	_, err := wb.conn.WriteMultipleRegisters(etrelRegMaxCurrent, 2, b)
	return err
}

// MaxCurrent implements the api.Charger interface
func (wb *Etrel) MaxCurrent(current int64) error {
	return wb.MaxCurrentMillis(float64(current))
}

var _ api.ChargerEx = (*Etrel)(nil)

// MaxCurrentMillis implements the api.ChargerEx interface
func (wb *Etrel) MaxCurrentMillis(current float64) error {
	if current < 6 {
		return fmt.Errorf("invalid current %.1f", current)
	}

	f := float32(current)

	err := wb.setCurrent(f)
	if err == nil {
		wb.current = f
	}

	return err
}

var _ api.Meter = (*Etrel)(nil)

// CurrentPower implements the api.Meter interface
func (wb *Etrel) CurrentPower() (float64, error) {
	b, err := wb.conn.ReadInputRegisters(etrelRegPower, 2)
	if err != nil {
		return 0, err
	}

	return float64(encoding.Float32(b) * 1e3), err
}

// var _ api.ChargeRater = (*Etrel)(nil)

// // ChargedEnergy implements the api.ChargeRater interface
// func (wb *Etrel) ChargedEnergy() (float64, error) {
// 	b, err := wb.conn.ReadInputRegisters(etrelRegSessionEnergy, 2)
// 	if err != nil {
// 		return 0, err
// 	}

// 	fmt.Printf("%.5f", encoding.Float32(b))

// 	return float64(encoding.Float32(b)), err
// }

var _ api.MeterCurrent = (*Etrel)(nil)

// Currents implements the api.MeterCurrent interface
func (wb *Etrel) Currents() (float64, float64, float64, error) {
	b, err := wb.conn.ReadInputRegisters(etrelRegCurrents, 6)
	if err != nil {
		return 0, 0, 0, err
	}

	var currents [3]float64
	for i := 0; i < 3; i++ {
		currents[i] = float64(encoding.Float32(b[4*i:]))
	}

	return currents[0], currents[1], currents[2], nil
}

var _ api.Diagnosis = (*Etrel)(nil)

// Diagnose implements the api.Diagnosis interface
func (wb *Etrel) Diagnose() {
	if b, err := wb.conn.ReadInputRegisters(etrelRegModel, 10); err == nil {
		fmt.Printf("Model:\t%s\n", b)
	}
	if b, err := wb.conn.ReadInputRegisters(etrelRegSerial, 10); err == nil {
		fmt.Printf("Serial:\t%s\n", b)
	}
	if b, err := wb.conn.ReadInputRegisters(etrelRegHWVersion, 5); err == nil {
		fmt.Printf("Hardware:\t%s\n", b)
	}
	if b, err := wb.conn.ReadInputRegisters(etrelRegSWVersion, 5); err == nil {
		fmt.Printf("Software:\t%s\n", b)
	}
}
