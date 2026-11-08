package app

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/louis49/merlinus/internal/library"
	"github.com/louis49/merlinus/internal/store"
)

// dossierAudio cree un dossier temporaire contenant des faux mp3 (contenu
// distinct par nom, > 1 Ko) et des fichiers a ignorer.
func dossierAudio(t *testing.T, nom string, fichiers ...string) string {
	t.Helper()
	d := filepath.Join(t.TempDir(), nom)
	if err := os.MkdirAll(filepath.Join(d, "sous-dossier"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range fichiers {
		data := append([]byte(f), bytes.Repeat([]byte{0}, 2048)...)
		if err := os.WriteFile(filepath.Join(d, f), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Ignores : sous-dossier (meme avec de l'audio) et fichier non audio.
	_ = os.WriteFile(filepath.Join(d, "sous-dossier", "x.mp3"), bytes.Repeat([]byte{1}, 2048), 0o644)
	_ = os.WriteFile(filepath.Join(d, "pochette.txt"), []byte("texte"), 0o644)
	return d
}

func pngDataURL(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes())
}

func reinitPerso(t *testing.T) *App {
	t.Helper()
	_ = store.Remove(ficPerso)
	return &App{}
}

func TestImporterDossierCreeUneSerie(t *testing.T) {
	a := reinitPerso(t)
	d := dossierAudio(t, "L'opéra de la lune", "02. Deux.mp3", "01. Un.mp3", "10 - Dix.mp3", "11 Onze.mp3")
	h, err := a.importerDossier(d)
	if err != nil {
		t.Fatal(err)
	}
	if h.Titre != "L'opéra de la lune" {
		t.Errorf("titre = %q", h.Titre)
	}
	var titres []string
	for _, p := range h.Pistes {
		titres = append(titres, p.Titre)
	}
	if want := []string{"Un", "Deux", "Dix", "Onze"}; strings.Join(titres, "|") != strings.Join(want, "|") {
		t.Fatalf("pistes = %v, attendu %v (tri par nom, numero retire)", titres, want)
	}
	l := a.ListerHistoiresPerso()
	if len(l) != 1 || l[0].UUID != h.UUID || len(l[0].Pistes) != 4 {
		t.Fatalf("registre : %d entree(s), attendu une seule carte serie de 4 pistes : %+v", len(l), l)
	}
	if err := uuidMediaValide(h.UUID); err != nil {
		t.Errorf("uuid de serie invalide : %v", err)
	}
	for _, p := range h.Pistes {
		if library.ExtensionAudio(p.UUID) != "mp3" {
			t.Errorf("audio de %q absent de la bibliotheque", p.Titre)
		}
	}
}

func TestImporterDossierSansAudio(t *testing.T) {
	a := reinitPerso(t)
	if _, err := a.importerDossier(dossierAudio(t, "vide")); err == nil {
		t.Fatal("un dossier sans audio doit etre refuse")
	}
	if len(a.ListerHistoiresPerso()) != 0 {
		t.Fatal("aucune carte attendue")
	}
}

// Re-importer le meme dossier met a jour la carte au lieu de la dupliquer, et
// garde le titre choisi par l'utilisateur.
func TestReimporterDossierSansDoublon(t *testing.T) {
	a := reinitPerso(t)
	d := dossierAudio(t, "Serie", "01. A.mp3", "02. B.mp3")
	h1, err := a.importerDossier(d)
	if err != nil {
		t.Fatal(err)
	}
	a.RenommerHistoirePerso(h1.UUID, "Mon titre")
	h2, err := a.importerDossier(d)
	if err != nil {
		t.Fatal(err)
	}
	l := a.ListerHistoiresPerso()
	if h2.UUID != h1.UUID || len(l) != 1 || l[0].Titre != "Mon titre" {
		t.Fatalf("re-import : %+v", l)
	}
}

// L'illustration de la serie sert aussi a chacune de ses histoires (la boite
// exige une image par histoire).
func TestIllustrationSeriePropagee(t *testing.T) {
	a := reinitPerso(t)
	h, err := a.importerDossier(dossierAudio(t, "Serie", "01. A.mp3", "02. B.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.DefinirIllustrationPerso(h.UUID, pngDataURL(t)); err != nil {
		t.Fatal(err)
	}
	serie := library.IllustrationJPEG(h.UUID)
	if len(serie) == 0 {
		t.Fatal("illustration de serie absente")
	}
	for _, p := range h.Pistes {
		if !bytes.Equal(library.IllustrationJPEG(p.UUID), serie) {
			t.Errorf("piste %q : illustration differente de celle de la serie", p.Titre)
		}
	}
	if l := a.ListerHistoiresPerso(); !l[0].Cover {
		t.Error("Cover attendu a vrai")
	}
}

func TestSupprimerSerie(t *testing.T) {
	a := reinitPerso(t)
	h, err := a.importerDossier(dossierAudio(t, "Serie", "01. A.mp3", "02. B.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SupprimerHistoirePerso(h.UUID); err != nil {
		t.Fatal(err)
	}
	if len(a.ListerHistoiresPerso()) != 0 {
		t.Fatal("carte encore presente")
	}
	for _, p := range h.Pistes {
		if library.ExtensionAudio(p.UUID) != "" {
			t.Errorf("audio de %q encore en bibliotheque", p.Titre)
		}
	}
}

// Une serie dont une histoire est dans l'arbre d'une boite est "referencee" :
// sa suppression est refusee.
func TestSerieReferencee(t *testing.T) {
	a := reinitPerso(t)
	h, err := a.importerDossier(dossierAudio(t, "Serie", "01. A.mp3", "02. B.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	mac := "02:aa:bb:cc:dd:ee"
	t.Cleanup(func() { _ = store.RemoveDir(dirTravailBoite(mac)) })
	travail := `[{"uuid":"11111111-1111-4111-8111-111111111111","title":"Cat","child":[{"uuid":"` + h.UUID +
		`","title":"Serie","child":[{"uuid":"` + h.Pistes[0].UUID + `","title":"A"}]}]}]`
	if err := store.Save(ficTravail(mac), []byte(travail)); err != nil {
		t.Fatal(err)
	}
	if l := a.ListerHistoiresPerso(); !l[0].Referencee {
		t.Fatal("serie attendue referencee")
	}
	if err := a.SupprimerHistoirePerso(h.UUID); cleErreur(err) != "err_perso_referencee" {
		t.Fatalf("suppression : %v, attendu err_perso_referencee", err)
	}
}

// Un fichier importe seul PUIS dans une serie (meme contenu) donne deux
// histoires distinctes : uuids differents (sinon etats melanges dans l'IHM et
// uuid en double dans la playlist), et supprimer l'une ne touche pas l'autre.
func TestFichierSeulEtDansSerieIndependants(t *testing.T) {
	a := reinitPerso(t)
	d := dossierAudio(t, "A tire d'aile", "01 Les gros crocodiles.mp3", "02 Autre.mp3")
	seul, err := a.importerChemin(filepath.Join(d, "01 Les gros crocodiles.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	serie, err := a.importerDossier(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range serie.Pistes {
		if p.UUID == seul.UUID {
			t.Fatalf("la piste %q reprend l'uuid de l'histoire importee seule", p.Titre)
		}
	}
	if err := a.SupprimerHistoirePerso(seul.UUID); err != nil {
		t.Fatal(err)
	}
	if library.ExtensionAudio(serie.Pistes[0].UUID) == "" {
		t.Fatal("supprimer l'histoire seule a retire l'audio de la piste de la serie")
	}
}
