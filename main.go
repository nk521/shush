package main

import (
	"log"
	"os"
	"strconv"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
	"golang.design/x/hotkey"
	"golang.design/x/hotkey/mainthread"
)

type Device struct {
	name   string
	device *wca.IMMDevice
}

var TotalDevices uint32

var Devices []Device

var MuteHotKey *hotkey.Hotkey
var ExitHotKey *hotkey.Hotkey

func hotkeyInit() {
	MuteHotKey = hotkey.New([]hotkey.Modifier{hotkey.ModCtrl, hotkey.ModShift}, hotkey.KeyO)
	err := MuteHotKey.Register()
	if err != nil {
		log.Panicln("hotkey: failed to register hotkey:", err)
		return
	}
	ExitHotKey = hotkey.New([]hotkey.Modifier{hotkey.ModCtrl, hotkey.ModShift}, hotkey.KeyU)
	err = ExitHotKey.Register()
	if err != nil {
		log.Panicln("hotkey: failed to register hotkey:", err)
		return
	}

}

func mute(index uint64, mmde *wca.IMMDeviceEnumerator) {

	var selectedDevice *wca.IMMDevice
	if index == 0 {
		if err := mmde.GetDefaultAudioEndpoint(wca.ECapture, wca.ECommunications, &selectedDevice); err != nil {
			return
		}
	} else {
		selectedDevice = Devices[index-1].device
	}
	defer selectedDevice.Release()

	var aev *wca.IAudioEndpointVolume
	if err := selectedDevice.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &aev); err != nil {
		return
	}
	defer aev.Release()

	var isMuted bool
	aev.GetMute(&isMuted)

	if err := aev.SetMute(!isMuted, nil); err != nil {
		return
	}
}

func checkArgs() uint64 {
	var errMsg string = "Second argument should be 0 for default device or any device id. Run `shush.exe list` to list device ids."
	if len(os.Args) < 2 {
		log.Panicln(errMsg)
	}
	deviceIndexInput, err := strconv.ParseUint(os.Args[2], 10, 32)
	if err != nil {
		log.Panicln(err)
	}

	if deviceIndexInput > uint64(TotalDevices) {
		log.Panicln(errMsg)
	}
	return deviceIndexInput
}

func main() {
	err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	if err != nil {
		log.Fatalf("Failed to initialize COM library: %v", err)
	}
	defer ole.CoUninitialize()

	var mmde *wca.IMMDeviceEnumerator
	if err = wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &mmde); err != nil {
		return
	}
	defer mmde.Release()

	var mmdc *wca.IMMDeviceCollection
	if err = mmde.EnumAudioEndpoints(wca.ECapture, wca.DEVICE_STATE_ACTIVE, &mmdc); err != nil {
		return
	}
	defer mmdc.Release()

	if err = mmdc.GetCount(&TotalDevices); err != nil {
		return
	}

	var tmpDevice *wca.IMMDevice
	var tmpDeviceProp *wca.IPropertyStore
	var tmpDevicePropVariant wca.PROPVARIANT
	var tmpDeviceName string

	for i := 0; i < int(TotalDevices); i++ {
		mmdc.Item(uint32(i), &tmpDevice)
		if err = tmpDevice.OpenPropertyStore(wca.STGM_READ, &tmpDeviceProp); err != nil {
			log.Panicln(err)
		}
		// v.GetId(&tmpName)
		if err = tmpDeviceProp.GetValue(&wca.PKEY_Device_FriendlyName, &tmpDevicePropVariant); err != nil {
			tmpDeviceName = "Unknown Device"
		} else {
			tmpDeviceName = tmpDevicePropVariant.String()
		}

		Devices = append(Devices, Device{
			name:   tmpDeviceName,
			device: tmpDevice,
		})
	}

	defer func() {
		for _, v := range Devices {
			v.device.Release()
		}
	}()

	if len(os.Args) < 2 {
		log.Println("Usage: `shush.exe [ list | mute | lmute ] [0 | 1 ...] `")
		os.Exit(-1)
	}

	switch os.Args[1] {
	case "list":
		{
			for x, v := range Devices {
				log.Println(x+1, "->", v.name)
			}
		}
	case "mute":
		{
			mute(checkArgs(), mmde)
		}
	case "lmute":
		{
			arg2 := checkArgs()
			mainthread.Init(hotkeyInit)
			defer func() {
				log.Println("Quitting")
				_ = MuteHotKey.Unregister()
				_ = ExitHotKey.Unregister()
			}()
			for {
				select {
				case <-MuteHotKey.Keydown():
					mute(arg2, mmde)
				case <-ExitHotKey.Keydown():
					return
				}
			}
		}
	}
}
