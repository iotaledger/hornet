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

	"github.com/gohornet/hornet/pkg/model/hornet"
	iotago "github.com/iotaledger/iota.go/v2"
)

type Vertex struct {
	MessageID string   `json:"id"`
	Parents   []string `json:"parents"`
}

// returns length amount random bytes
func randBytes(length int) []byte {
	var b []byte
	for i := 0; i < length; i++ {
		b = append(b, byte(rand.Intn(256)))
	}
	return b
}

func randMessageID() hornet.MessageID {
	return hornet.MessageID(randBytes(iotago.MessageIDLength))
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
		v := Vertex{MessageID: randMessageID().ToHex()}
		if i <= getFromLast {
			// only one parent at the beginning
			v.Parents = hornet.MessageIDs{hornet.NullMessageID()}.ToHex()
			vertices = append(vertices, v)
			continue
		}

		l := len(vertices)
		parents := hornet.MessageIDs{}
		for j := 2; j <= 2+rand.Intn(7); j++ {
			msgID, err := hornet.MessageIDFromHex(vertices[l-1-rand.Intn(getFromLast)].MessageID)
			assert.NoError(t, err)
			parents = append(parents, msgID)
		}
		parents = parents.RemoveDupsAndSortByLexicalOrder()
		v.Parents = parents.ToHex()
		vertices = append(vertices, v)
	}

	verticesJSON, err := json.Marshal(vertices)
	assert.NoError(t, err)
	assert.NoError(t, temp.Execute(f, struct {
		Vertices template.HTML
	}{Vertices: template.HTML(verticesJSON)}))

}
