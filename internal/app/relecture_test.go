package app

import (
	"encoding/json"
	"testing"

	"github.com/louis49/merlinus/internal/playlist"
	"github.com/louis49/merlinus/internal/store"
)

// Boite modifiee hors de Merlinus (app officielle) : l'arbre en cache et le
// travail mentionnent H3, que la boite n'a plus. La relecture remet l'arbre
// local au contenu reel, solde le travail et met a jour inventaire et favoris.
func TestRelireBoite(t *testing.T) {
	mac := boiteVierge(t, "02:00:00:00:5e:01")
	b := nouvelleFausseBoite(t)
	b.poserPlaylist(arbreReel()) // H1, H2 : H3 a disparu
	b.fichiers[pH1+".mp3"] = contenu("m", 3000)
	b.fichiers[pH1+".jpg"] = contenu("j", 500)
	b.libre = 4242

	perime := []*playlist.Node{{UUID: pCat, Title: "Cat", Children: []*playlist.Node{{UUID: pH3, Title: "H3"}}}}
	d, _ := json.Marshal(perime)
	for rel, data := range map[string][]byte{
		ficArbre(mac):   d,
		ficTravail(mac): []byte(`[{"uuid":"` + pCat + `","title":"Cat","child":[{"uuid":"` + pH3 + `","title":"H3"}]}]`),
		ficTailles(mac): []byte(`{"` + pH3 + `.mp3":9999}`),
	} {
		if err := store.Save(rel, data); err != nil {
			t.Fatal(err)
		}
	}
	a := NewApp()
	a.majMeta(BoiteMeta{MAC: mac})

	if err := a.relireBoite(b.connecter(), mac); err != nil {
		t.Fatal(err)
	}
	arbre, err := a.ArbreBoite(mac)
	if err != nil {
		t.Fatal(err)
	}
	f := playlist.Feuilles(arbre)
	if !f[pH1] || !f[pH2] || f[pH3] {
		t.Errorf("arbre relu = %v", f)
	}
	if store.Has(ficTravail(mac)) {
		t.Error("travail perime conserve")
	}
	tailles := taillesParUUID(mac)
	if tailles[pH3] != 0 || tailles[pH1] != 3500 {
		t.Errorf("inventaire non relu : %v", tailles)
	}
	for _, m := range a.lireRegistre() {
		if memeMAC(m.MAC, mac) && m.StockageLibre != 4242 {
			t.Errorf("stockage non releve : %+v", m)
		}
	}
	if !store.Has(cheminAffichage(mac, pH1)) {
		t.Error("illustration non mise en cache")
	}
	// Lecture seule : rien d'ecrit sur la boite, pas de fin de synchro.
	if ops := b.operations("upload "); len(ops) != 0 {
		t.Errorf("ecritures inattendues : %v", ops)
	}
}
