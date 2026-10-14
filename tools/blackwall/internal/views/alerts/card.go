package alerts

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/indexer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
)

type card struct{ alert indexer.Alert }

func newCard(a indexer.Alert, width, height int) *card { return &card{alert: a} }
func (c *card) setSize(width, height int)              {}
func (c *card) handleKey(tea.KeyMsg)                   {}
func (c *card) hints() []footer.Hint                   { return []footer.Hint{{Key: "esc", Desc: "back"}} }
func (c *card) view() string                           { return c.alert.Rule.Description }
