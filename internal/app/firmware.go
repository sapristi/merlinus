package app

import (
	"log"

	"github.com/louis49/merlinus/internal/device"
)

// firmwareMinimum : seule version validee en reel (format pl.bin / pl_enc.bin de
// 96 octets, chiffrement, clean_sd.cmd). Les versions anterieures (ex. 2.1.16,
// issue #2) ne publient pas de playlist lisible par Merlinus.
var firmwareMinimum = device.Firmware{Major: 2, Minor: 1, Patch: 23}

// controlerFirmware lit la version (op5) d'une boite a associer. Bloquant si la
// version est illisible ou anterieure au minimum ; une version plus recente est
// acceptee (non testee : simple ligne de journal).
func controlerFirmware(c *device.Client) (device.Firmware, error) {
	v, err := c.FirmwareVersion()
	if err != nil {
		return v, errDetail("err_firmware_illisible", err)
	}
	switch v.Comparer(firmwareMinimum) {
	case -1:
		// Certaines boites annoncent une version d'une autre lignee (ex. 1.3.18)
		// tout en utilisant le format 2.1.23 : la preuve est un pl_enc.bin
		// dechiffrable avec leur MAC.
		if formatChiffre(c) {
			log.Printf("firmware %s anterieur a %s mais pl_enc.bin au format 2.1.23 : poursuite", v, firmwareMinimum)
			return v, nil
		}
		return v, errIHM("err_firmware_ancien", "version", v.String(), "minimum", firmwareMinimum.String())
	case 1:
		log.Printf("firmware %s plus recent que la version validee %s : poursuite", v, firmwareMinimum)
	}
	return v, nil
}

// formatChiffre : vrai si la boite porte un pl_enc.bin dechiffrable avec sa MAC.
func formatChiffre(c *device.Client) bool {
	mac, err := c.MACBytes()
	if err != nil {
		return false
	}
	enc, err := c.GetFileBytes("pl_enc.bin")
	if err != nil {
		return false
	}
	_, err = device.DechiffrerPlaylist(mac, enc)
	return err == nil
}
