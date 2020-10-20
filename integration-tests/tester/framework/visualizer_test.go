package framework

import (
	"encoding/json"
	"fmt"
	"html/template"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/gohornet/hornet/pkg/model/hornet"
	iotago "github.com/iotaledger/iota.go"
	"github.com/stretchr/testify/assert"
)

type Vertex struct {
	MessageID        string `json:"id"`
	Parent1MessageID string `json:"parent1"`
	Parent2MessageID string `json:"parent2"`
}

// returns length amount random bytes
func randBytes(length int) []byte {
	var b []byte
	for i := 0; i < length; i++ {
		b = append(b, byte(rand.Intn(256)))
	}
	return b
}

func rand32ByteHash() [iotago.TransactionIDLength]byte {
	var h [iotago.TransactionIDLength]byte
	b := randBytes(32)
	copy(h[:], b)
	return h
}

func randMessageID() *hornet.MessageID {
	messageID := hornet.MessageID(rand32ByteHash())
	return &messageID
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
		v := Vertex{MessageID: randMessageID().Hex()}
		if i <= getFromLast {
			v.Parent1MessageID = hornet.GetNullMessageID().Hex()
			v.Parent2MessageID = hornet.GetNullMessageID().Hex()
			vertices = append(vertices, v)
			continue
		}
		l := len(vertices)
		v.Parent1MessageID = vertices[l-1-rand.Intn(getFromLast)].MessageID
		v.Parent2MessageID = vertices[l-1-rand.Intn(getFromLast)].MessageID
		vertices = append(vertices, v)
	}

	verticesJSON, err := json.Marshal(vertices)
	assert.NoError(t, err)
	assert.NoError(t, temp.Execute(f, struct {
		Vertices template.HTML
	}{Vertices: template.HTML(verticesJSON)}))

}
