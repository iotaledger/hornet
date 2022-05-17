package framework

import (
	"encoding/json"
	"fmt"
	"html/template"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	iotago "github.com/iotaledger/iota.go/v3"
)

type Vertex struct {
	BlockID string   `json:"id"`
	Parents []string `json:"parents"`
}

// returns length amount random bytes
func randBytes(length int) []byte {
	var b []byte
	for i := 0; i < length; i++ {
		b = append(b, byte(rand.Intn(256)))
	}
	return b
}

func randBlockID() iotago.BlockID {
	blockID := iotago.BlockID{}
	copy(blockID[:], randBytes(iotago.BlockIDLength))
	return blockID
}

func TestVisualizer(t *testing.T) {
	f, err := os.OpenFile(fmt.Sprintf("vis_%d.html", time.Now().Unix()), os.O_RDWR|os.O_CREATE, 0666)
	assert.NoError(t, err)
	defer func() { _ = f.Close() }()

	temp, err := template.New("vis").ParseFiles("./vis_temp.html")
	assert.NoError(t, err)

	var vertices []Vertex
	const getFromLast = 30
	for i := 0; i < 1000; i++ {
		blockID := randBlockID()
		v := Vertex{BlockID: blockID.ToHex()}
		if i <= getFromLast {
			// only one parent at the beginning
			v.Parents = iotago.BlockIDs{iotago.EmptyBlockID()}.ToHex()
			vertices = append(vertices, v)
			continue
		}

		l := len(vertices)
		parents := iotago.BlockIDs{}
		for j := 2; j <= 2+rand.Intn(7); j++ {
			blockID, err := iotago.BlockIDFromHexString(vertices[l-1-rand.Intn(getFromLast)].BlockID)
			assert.NoError(t, err)
			parents = append(parents, blockID)
		}
		parents = parents.RemoveDupsAndSort()
		v.Parents = parents.ToHex()
		vertices = append(vertices, v)
	}

	verticesJSON, err := json.Marshal(vertices)
	assert.NoError(t, err)
	assert.NoError(t, temp.Execute(f, struct {
		Vertices template.HTML
	}{Vertices: template.HTML(verticesJSON)}))

}
