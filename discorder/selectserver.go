package discorder

import (
	"fmt"
	"github.com/jonas747/discorder/common"
	//"github.com/jonas747/discorder/common"
	"github.com/jonas747/discorder/ui"
	"github.com/jonas747/discordgo"
	"log"
)

const (
	ServerSelectTitle  = "Select a server"
	ServerSelectFooter = "(Space) Toggle whole server, (enter) select"
)

type ServerSelectWindow struct {
	*ui.BaseEntity
	App         *App
	menuWindow  *ui.MenuWindow
	messageView *MessageView
	viewManager *ViewManager
	Layer       int
}

func NewSelectServerWindow(app *App, messageView *MessageView, layer int) *ServerSelectWindow {
	ssw := &ServerSelectWindow{
		BaseEntity:  &ui.BaseEntity{},
		App:         app,
		messageView: messageView,
		Layer:       layer,
	}

	menuWindow := ui.NewMenuWindow(layer, app.ViewManager.UIManager)

	menuWindow.Transform.AnchorMax = common.NewVector2F(1, 1)
	menuWindow.Transform.Top = 1
	menuWindow.Transform.Bottom = 2

	menuWindow.Window.Footer = ServerSelectFooter
	menuWindow.Window.Title = ServerSelectTitle

	app.ApplyThemeToMenu(menuWindow)

	ssw.menuWindow = menuWindow
	ssw.Transform.AddChildren(menuWindow)

	ssw.Transform.AnchorMin = common.NewVector2F(0.1, 0)
	ssw.Transform.AnchorMax = common.NewVector2F(0.9, 1)

	ssw.GenMenu()
	//height := float32(menuWindow.OptionsHeight() + 5)

	app.ViewManager.UIManager.AddWindow(ssw)

	return ssw
}

func (ssw *ServerSelectWindow) GenMenu() {
	state := ssw.App.session.State
	state.RLock()
	defer state.RUnlock()

	if len(state.Guilds) < 1 {
		log.Println("No guilds, probably starting up still...")
		return
	}

	// Generate guild options
	rootOptions := make([]*ui.MenuItem, len(state.Guilds)+1)
	for k, guild := range state.Guilds {
		guildOption := &ui.MenuItem{
			Name:     guild.Name,
			IsDir:    true,
			UserData: guild,
			Info:     fmt.Sprintf("Members: %d\nID:%s", len(guild.Members), guild.ID),
			Children: make([]*ui.MenuItem, len(guild.Channels)),
		}

		// Generate chanel options
		for i, channel := range guild.Channels {
			marked := false
			for _, listening := range ssw.messageView.Channels {
				if listening == channel.ID {
					marked = true
					break
				}
			}
			channelOption := &ui.MenuItem{
				Name:     "#" + channel.Name,
				UserData: channel,
				Info:     fmt.Sprintf("Topic %s", channel.Topic),
				Marked:   marked,
			}
			guildOption.Children[i] = channelOption
			if marked {
				guildOption.Marked = true
			}
		}
		rootOptions[k+1] = guildOption
	}
	rootOptions[0] = &ui.MenuItem{
		Name:        "Direct Messages",
		Highlighted: true,
		IsDir:       true,
		Children:    make([]*ui.MenuItem, len(state.PrivateChannels)),
	}

	for i, channel := range state.PrivateChannels {
		marked := false
		if ssw.messageView.ShowAllPrivate {
			marked = true
		} else {
			for _, listening := range ssw.messageView.Channels {
				if listening == channel.ID {
					marked = true
					break
				}
			}
		}

		channelOption := &ui.MenuItem{
			Name:     GetChannelNameOrRecipient(channel),
			UserData: channel,
			Info:     fmt.Sprintf("Topic %s", channel.Topic),
			Marked:   marked,
		}
		if marked {
			rootOptions[0].Marked = true
		}
		rootOptions[0].Children[i] = channelOption
	}

	ssw.menuWindow.SetOptions(rootOptions)
}

func (ssw *ServerSelectWindow) Destroy() {
	ssw.App.ViewManager.UIManager.RemoveWindow(ssw)
	ssw.DestroyChildren()
}

func (ssw *ServerSelectWindow) Back() {
	if len(ssw.menuWindow.CurDir) < 1 {
		ssw.Transform.Parent.RemoveChild(ssw, true)
	} else {
		ssw.menuWindow.Back()
	}
}

func (ssw *ServerSelectWindow) Select() {
	element := ssw.menuWindow.GetHighlighted()
	if element == nil {
		return
	}

	if element.IsDir {
		ssw.menuWindow.Select()
		return
	}

	if element.UserData == nil {
		return
	}

	cast, ok := element.UserData.(*discordgo.Channel)
	if !ok {
		return
	}

	log.Println("Selected ", GetChannelNameOrRecipient(cast))
	ssw.App.ViewManager.talkingChannel = cast.ID
}

func (ssw *ServerSelectWindow) Toggle() {
	element := ssw.menuWindow.GetHighlighted()
	if element == nil {
		return
	}

	if element.UserData == nil {
		return
	}

	switch t := element.UserData.(type) {
	case *discordgo.Channel:
		ssw.ToggleChannel(t, element)
	case *discordgo.Guild:
		ssw.ToggleGuild(t, element)
	case string:
		ssw.ToggleDirectMessages()
	}
}

func (ssw *ServerSelectWindow) ToggleGuild(guild *discordgo.Guild, element *ui.MenuItem) {

	toggleTo := true
OUTER:
	for _, v := range guild.Channels {
		for _, c := range ssw.messageView.Channels {
			if v.ID == c {
				toggleTo = false
				break OUTER
			}
		}
	}

	element.Marked = toggleTo
	for _, v := range guild.Channels {
		if v.Type != "text" && !v.IsPrivate {
			continue
		}

		if toggleTo {
			ssw.messageView.AddChannel(v.ID)
		} else {
			ssw.messageView.RemoveChannel(v.ID)
		}
	}

	ssw.menuWindow.RunFunc(func(item *ui.MenuItem) bool {
		cast, ok := item.UserData.(*discordgo.Channel)
		if ok && cast.GuildID == guild.ID {
			item.Marked = toggleTo
		}
		return true
	})

	ssw.messageView.ShowAllPrivate = toggleTo
	ssw.menuWindow.Dirty = true
}

func (ssw *ServerSelectWindow) ToggleChannel(channel *discordgo.Channel, element *ui.MenuItem) {
	element.Marked = !element.Marked

	//Reflect changes to messageview
	if element.Marked {
		ssw.messageView.AddChannel(channel.ID)
	} else {
		ssw.messageView.RemoveChannel(channel.ID)
	}
	ssw.menuWindow.Dirty = true

	if channel.IsPrivate {
		all := true
		ssw.menuWindow.RunFunc(func(item *ui.MenuItem) bool {
			cast, ok := item.UserData.(*discordgo.Channel)
			if ok && cast.IsPrivate {
				if !item.Marked {
					all = false
					return false
				}
			}
			return true
		})

		if all {
			ssw.App.ViewManager.mv.ShowAllPrivate = true
		} else {
			ssw.App.ViewManager.mv.ShowAllPrivate = false
		}
	}
}

func (ssw *ServerSelectWindow) ToggleDirectMessages() {

}
