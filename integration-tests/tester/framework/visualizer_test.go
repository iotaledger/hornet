package framework

import (
	"encoding/json"
	"fmt"
	"html/template"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/stretchr/testify/assert"
)

type Vertex struct {
	ID      string `json:"id"`
	Parent1 string `json:"parent1"`
	Parent2 string `json:"parent2"`
}

func randHash() trinary.Trytes {
	var h string
	for i := 0; i < consts.HashTrytesSize; i++ {
		h += string(consts.TryteAlphabet[rand.Intn(len(consts.TryteAlphabet))])
	}
	return h
}

func TestVisualizer(t *testing.T) {
	f, err := os.OpenFile(fmt.Sprintf("vis_%d.html", time.Now().Unix()), os.O_RDWR|os.O_CREATE, 0666)
	assert.NoError(t, err)
	defer f.Close()

	temp, err := template.New("vis").ParseFiles("./vis_temp.html")
	assert.NoError(t, err)

	var vertices []Vertex
	const getFromLast = 30
	for i := 0; i < 10000; i++ {
		v := Vertex{ID: randHash()}
		if i <= getFromLast {
			v.Parent1 = consts.NullHashTrytes
			v.Parent2 = consts.NullHashTrytes
			vertices = append(vertices, v)
			continue
		}
		l := len(vertices)
		v.Parent1 = vertices[l-1-rand.Intn(getFromLast)].ID
		v.Parent2 = vertices[l-1-rand.Intn(getFromLast)].ID
		vertices = append(vertices, v)
	}

	verticesJSON, err := json.Marshal(vertices)
	assert.NoError(t, err)
	assert.NoError(t, temp.Execute(f, struct {
		Vertices template.HTML
	}{Vertices: template.HTML(verticesJSON)}))

}
