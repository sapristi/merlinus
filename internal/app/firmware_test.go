package app

import "testing"

// Controle du firmware a l'association : bloquant sous 2.1.23 ou si op5 est
// illisible, accepte a partir de 2.1.23 (y compris plus recent, non teste).
func TestControlerFirmware(t *testing.T) {
	cas := []struct {
		nom     string
		reponse []byte
		version string
		cle     string // "" = accepte
	}{
		{"valide", []byte{2, 1, 23, 0}, "2.1.23", ""},
		{"plus recent", []byte{2, 1, 24, 0}, "2.1.24", ""},
		{"majeure suivante", []byte{3, 0, 0, 0}, "3.0.0", ""},
		{"issue 2", []byte{2, 1, 16, 0}, "2.1.16", "err_firmware_ancien"},
		{"mineure anterieure", []byte{2, 0, 99, 0}, "2.0.99", "err_firmware_ancien"},
		{"reponse courte", []byte{2, 1}, "", "err_firmware_illisible"},
	}
	for _, tc := range cas {
		t.Run(tc.nom, func(t *testing.T) {
			b := nouvelleFausseBoite(t)
			b.firmware = tc.reponse
			v, err := controlerFirmware(b.connecter())
			if got := cleErreur(err); got != tc.cle {
				t.Fatalf("erreur = %v, cle attendue %q", err, tc.cle)
			}
			if tc.version != "" && v.String() != tc.version {
				t.Errorf("version = %s, attendu %s", v, tc.version)
			}
		})
	}
}

// Une boite qui annonce une version plus ancienne (ex. 1.3.18, lignee
// materielle distincte) mais porte un pl_enc.bin dechiffrable avec sa MAC
// utilise le format 2.1.23 : acceptee. Sans pl_enc.bin lisible : refusee.
func TestControlerFirmwareFormatChiffre(t *testing.T) {
	t.Run("pl_enc.bin dechiffrable", func(t *testing.T) {
		b := nouvelleFausseBoite(t)
		b.firmware = []byte{1, 3, 18, 0}
		b.poserPlaylist(arbreReel())
		v, err := controlerFirmware(b.connecter())
		if err != nil {
			t.Fatalf("erreur = %v, attendu accepte", err)
		}
		if v.String() != "1.3.18" {
			t.Errorf("version = %s, attendu 1.3.18", v)
		}
	})
	t.Run("sans pl_enc.bin", func(t *testing.T) {
		b := nouvelleFausseBoite(t)
		b.firmware = []byte{1, 3, 18, 0}
		if _, err := controlerFirmware(b.connecter()); cleErreur(err) != "err_firmware_ancien" {
			t.Fatalf("erreur = %v, attendu err_firmware_ancien", err)
		}
	})
	t.Run("pl_enc.bin d'une autre boite", func(t *testing.T) {
		b := nouvelleFausseBoite(t)
		b.firmware = []byte{1, 3, 18, 0}
		b.poserPlaylist(arbreReel())
		b.mac = []byte{0x02, 0x99, 0x99, 0x99, 0x99, 0x99} // op30 ne correspond plus a la cle
		if _, err := controlerFirmware(b.connecter()); cleErreur(err) != "err_firmware_ancien" {
			t.Fatalf("erreur = %v, attendu err_firmware_ancien", err)
		}
	})
}
