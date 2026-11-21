package app

import (
	"encoding/json"
	"log"
	"time"

	"github.com/louis49/merlinus/internal/device"
	"github.com/louis49/merlinus/internal/playlist"
	"github.com/louis49/merlinus/internal/store"
)

// RelireBoite recalcule l'etat local d'une boite DEJA connue a partir de son
// contenu reel : playlist (arbre), inventaire des fichiers (tailles), stockage,
// favoris et illustrations manquantes. Sert quand la boite a ete modifiee hors
// de Merlinus (synchro par l'app officielle, qui peut retirer nos ajouts) : sans
// relecture, l'arbre en cache resterait celui de la derniere publication.
//
// Lecture seule cote boite (aucune ecriture, pas de EndSync : la boite reste en
// mode point d'acces). Les editions en cours (arbre de travail) sont abandonnees :
// elles portaient sur l'ancien arbre et pourraient designer comme "deja sur la
// boite" des contenus qui n'y sont plus. L'IHM demande confirmation avant.
func (a *App) RelireBoite(mac string) error {
	debut := time.Now()
	log.Printf("relecture %s : debut", mac)
	err := a.connecterPourEcrire(mac, func(c *device.Client) error { return a.relireBoite(c, mac) })
	if err != nil {
		log.Printf("relecture %s : echec apres %s : %v", mac, time.Since(debut).Round(time.Second), err)
	} else {
		log.Printf("relecture %s : terminee en %s", mac, time.Since(debut).Round(time.Second))
	}
	return err
}

func (a *App) relireBoite(c *device.Client, mac string) error {
	a.emitProgres("prog_lecture_boite", 0)
	items, bin, err := a.lirePlaylistBoite(c)
	if err != nil {
		return errDetail("err_lecture_playlist", err)
	}
	arbre, err := playlist.BuildTree(items)
	if err != nil {
		return errDetail("err_reconstruction_arbre", err)
	}
	rapport := playlist.Summarize(items, len(bin))

	// Inventaire des tailles reelles (op11/op12) : tolerant, comme a l'association.
	a.emitProgres("prog_inventaire", 25)
	c.SetTimeout(30 * time.Second)
	if fichiers, err := c.ListerFichiers(nil); err == nil {
		total, libre := uint32(0), uint32(0)
		if st := a.noterStockage(c, mac); st != nil {
			total, libre = st.Total, st.Libre
		}
		a.enregistrerTailles(mac, fichiers, total, libre)
	} else {
		log.Printf("relecture %s : inventaire des fichiers indisponible : %v", mac, err)
		a.noterStockage(c, mac)
	}

	// Favoris : fav.bin absent ou illisible -> liste persistee inchangee.
	if fb, err := c.GetFileBytes("fav.bin"); err == nil {
		if d, e := json.Marshal(playlist.FavorisDepuisBin(fb)); e == nil {
			_ = store.Save(ficFavoris(mac), d)
		}
	}

	// Illustrations : seules celles absentes du cache sont telechargees.
	a.emitProgres("prog_illustrations", 60)
	c.SetTimeout(15 * time.Second)
	a.cacherImagesArbre(c, arbre, mac)

	if err := a.persister(mac, arbre, rapport); err != nil {
		return errDetail("err_assoc_enregistrement", err)
	}
	store.Remove(ficTravail(mac)) // etat frais : les editions repartent de l'arbre relu
	a.emitProgres("prog_relecture_finie", 100)
	log.Printf("relecture %s : %d categories, %d dossiers, %d histoires",
		mac, rapport.Categories, rapport.Dossiers, rapport.Histoires)
	return nil
}
