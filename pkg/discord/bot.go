package discord

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"osrs-flipping/pkg/config"
	"osrs-flipping/pkg/llm"
	"osrs-flipping/pkg/logging"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

// Bot represents the Discord bot instance
type Bot struct {
	session          *discordgo.Session
	config           *config.DiscordConfig
	logger           *logging.Logger
	channelID        string
	mu               sync.RWMutex
	ready            bool
	lastCommandTime  time.Time
	commandsReceived int64
}

// NewBot creates a new Discord bot instance
func NewBot(cfg *config.DiscordConfig, logger *logging.Logger) (*Bot, error) {
	session, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	bot := &Bot{
		session:   session,
		config:    cfg,
		logger:    logger,
		channelID: cfg.ChannelID,
	}

	// Add event handlers
	session.AddHandler(bot.onReady)
	session.AddHandler(bot.onMessageCreate)

	// Set intents
	session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages

	return bot, nil
}

// Start starts the Discord bot
func (b *Bot) Start(ctx context.Context) error {
	b.logger.WithDiscord().Info("Starting Discord bot")

	if err := b.session.Open(); err != nil {
		return fmt.Errorf("failed to open Discord session: %w", err)
	}

	// Wait for ready state or context cancellation
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(30 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for Discord bot to be ready")
		case <-ticker.C:
			b.mu.RLock()
			ready := b.ready
			b.mu.RUnlock()
			if ready {
				b.logger.WithDiscord().Info("Discord bot is ready and connected")
				return nil
			}
		}
	}
}

// Stop stops the Discord bot
func (b *Bot) Stop() error {
	b.logger.WithDiscord().Info("Stopping Discord bot")
	return b.session.Close()
}

// SendMessage sends a message to the configured channel
func (b *Bot) SendMessage(content string) (*discordgo.Message, error) {
	if content == "" {
		return nil, fmt.Errorf("message content cannot be empty")
	}

	// Discord has a 2000 character limit
	if len(content) > 2000 {
		return b.sendLongMessage(content)
	}

	message, err := b.session.ChannelMessageSend(b.channelID, content)
	if err != nil {
		b.logger.DiscordError("send_message", err)
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	b.logger.DiscordMessage(b.channelID, message.ID, len(content))
	return message, nil
}

// SendEmbed sends an embedded message to the configured channel
func (b *Bot) SendEmbed(embed *discordgo.MessageEmbed) (*discordgo.Message, error) {
	message, err := b.session.ChannelMessageSendEmbed(b.channelID, embed)
	if err != nil {
		b.logger.DiscordError("send_embed", err)
		return nil, fmt.Errorf("failed to send embed: %w", err)
	}

	b.logger.DiscordMessage(b.channelID, message.ID, len(embed.Description))
	return message, nil
}

// SendTradingAnalysis sends a formatted trading analysis message
func (b *Bot) SendTradingAnalysis(jobName, analysis string, itemCount int) error {
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf(" %s", jobName),
		Description: analysis,
		Color:       0x00ff00, // Green
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Analyzed %d items • Generated at", itemCount),
		},
		Author: &discordgo.MessageEmbedAuthor{
			Name:    "osrs-flips",
			IconURL: "https://oldschool.runescape.wiki/images/c/c9/Coins_10000.png",
		},
	}

	_, err := b.SendEmbed(embed)
	return err
}

// SendError sends an error message to the configured channel
func (b *Bot) SendError(jobName string, err error) error {
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("❌ Error in Job: %s", jobName),
		Description: fmt.Sprintf("```\n%s\n```", err.Error()),
		Color:       0xff0000, // Red
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Error occurred at",
		},
	}

	_, sendErr := b.SendEmbed(embed)
	return sendErr
}

// sendLongMessage splits long messages into multiple Discord messages
func (b *Bot) sendLongMessage(content string) (*discordgo.Message, error) {
	const maxLength = 1900 // Leave some buffer for Discord

	// Use the LLM text splitter for consistent message handling
	splitter := llm.NewTextSplitter(maxLength)
	chunks := splitter.SplitTextWithParts(content)

	var firstMessage *discordgo.Message
	for i, chunk := range chunks {
		message, err := b.session.ChannelMessageSend(b.channelID, chunk)
		if err != nil {
			b.logger.DiscordError("send_long_message", err)
			return firstMessage, fmt.Errorf("failed to send message part %d: %w", i+1, err)
		}

		if i == 0 {
			firstMessage = message
		}

		b.logger.DiscordMessage(b.channelID, message.ID, len(chunk))

		// Add a small delay between messages to avoid rate limiting
		if i < len(chunks)-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return firstMessage, nil
}

// SendLongAnalysis sends long trading analysis messages, splitting them if necessary
// This method is optimized for trading analysis content and preserves full content
func (b *Bot) SendLongAnalysis(jobName, analysis string, footerText string, itemCount int) error {
	// const embedDescLimit = 4096 // Discord embed description limit

	// // If analysis fits in embed, use the existing method
	// if len(analysis) <= embedDescLimit {
	// 	return b.SendTradingAnalysis(jobName, analysis, itemCount)
	// }

	// For very long analysis, split and send as multiple messages
	splitter := llm.NewTextSplitter(3800) // Leave buffer for Discord
	chunks := splitter.SplitTextWithParts(analysis)

	for i, chunk := range chunks {
		var embed *discordgo.MessageEmbed

		if i == 0 {
			// First message gets the full header
			embed = &discordgo.MessageEmbed{
				Title:       fmt.Sprintf(" %s", jobName),
				Description: chunk,
				Color:       0x00ff00, // Green
				Author: &discordgo.MessageEmbedAuthor{
					Name:    "osrs-flips 🎯",
					IconURL: "https://oldschool.runescape.wiki/images/c/c9/Coins_10000.png",
				},
			}
		} else {
			// Subsequent messages are simpler
			embed = &discordgo.MessageEmbed{
				Title:       fmt.Sprintf(" %s (continued)", jobName),
				Description: chunk,
				Color:       0x00ff00, // Green
				Footer: &discordgo.MessageEmbedFooter{
					Text: footerText,
				},
				Author: &discordgo.MessageEmbedAuthor{
					Name:    "osrs-flips 🎯",
					IconURL: "https://oldschool.runescape.wiki/images/c/c9/Coins_10000.png",
				},
			}
		}

		if _, err := b.SendEmbed(embed); err != nil {
			return fmt.Errorf("failed to send analysis part %d: %w", i+1, err)
		}

		// Add delay between messages to avoid rate limiting
		if i < len(chunks)-1 {
			time.Sleep(150 * time.Millisecond)
		}
	}

	return nil
}

// onReady handles the ready event
func (b *Bot) onReady(s *discordgo.Session, event *discordgo.Ready) {
	b.mu.Lock()
	b.ready = true
	b.mu.Unlock()

	b.logger.WithDiscord().WithFields(map[string]interface{}{
		"bot_user_id": event.User.ID,
		"guild_count": len(event.Guilds),
	}).Info("Discord bot ready")

	// Set bot status
	err := s.UpdateGameStatus(0, "🎯 OSRS Trading Analysis")
	if err != nil {
		b.logger.WithDiscord().WithError(err).Warn("Failed to set bot status")
	}
}

// onMessageCreate handles incoming messages (for potential bot commands)
func (b *Bot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Only respond in the configured channel
	if m.ChannelID != b.channelID {
		return
	}

	// Simple command handling (can be extended)
	if strings.HasPrefix(m.Content, "!osrs") {
		// Handle commands asynchronously to prevent blocking the Discord event loop
		go b.handleCommand(s, m)
	}
}

// handleCommand handles bot commands
func (b *Bot) handleCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Add recovery to prevent panics from crashing the bot
	defer func() {
		if r := recover(); r != nil {
			b.logger.WithDiscord().WithField("panic", r).Error("Command handler panic recovered")

			embed := &discordgo.MessageEmbed{
				Title:       "❌ Command Error",
				Description: "An unexpected error occurred while processing your command.",
				Color:       0xff0000,
				Timestamp:   time.Now().Format(time.RFC3339),
			}
			if _, err := s.ChannelMessageSendEmbed(m.ChannelID, embed); err != nil {
				b.logger.WithDiscord().WithError(err).Error("Failed to send error embed")
			}
		}
	}() // Update command tracking
	b.mu.Lock()
	b.lastCommandTime = time.Now()
	b.commandsReceived++
	commandCount := b.commandsReceived
	b.mu.Unlock()

	parts := strings.Fields(m.Content)
	if len(parts) < 2 {
		return
	}

	command := strings.ToLower(parts[1])

	b.logger.WithDiscord().WithField("user_id", m.Author.ID).WithFields(logrus.Fields{
		"command":       command,
		"message":       m.Content,
		"command_count": commandCount,
	}).Info("Processing bot command")

	switch command {
	case "status":
		b.mu.RLock()
		lastCommand := b.lastCommandTime
		totalCommands := b.commandsReceived
		b.mu.RUnlock()

		uptime := time.Since(lastCommand)
		uptimeStr := uptime.Truncate(time.Second).String() + " ago"
		if uptime > 24*time.Hour {
			uptimeStr = "More than 24h ago" // Reset if unrealistic
		}

		embed := &discordgo.MessageEmbed{
			Title: "🟢 osrs-flips Status",
			Description: fmt.Sprintf("Bot is running and ready to analyze trading opportunities!\n\n"+
				"📊 **Statistics:**\n"+
				"• Total commands processed: %d\n"+
				"• Last command: %s\n",
				totalCommands,
				uptimeStr),
			Color:     0x00ff00,
			Timestamp: time.Now().Format(time.RFC3339),
		}
		if _, err := s.ChannelMessageSendEmbed(m.ChannelID, embed); err != nil {
			b.logger.WithDiscord().WithError(err).Error("Failed to send status command response")
		}

	case "help":
		embed := &discordgo.MessageEmbed{
			Title: "🎯 osrs-flips Commands",
			Description: "Available commands:\n" +
				"`!osrs status` - Check bot status\n" +
				"`!osrs help` - Show this help message\n" +
				"`!osrs ping` - Test bot responsiveness\n",
			Color:     0x0099ff,
			Timestamp: time.Now().Format(time.RFC3339),
		}
		if _, err := s.ChannelMessageSendEmbed(m.ChannelID, embed); err != nil {
			b.logger.WithDiscord().WithError(err).Error("Failed to send help command response")
		}

	case "ping":
		startTime := time.Now()
		embed := &discordgo.MessageEmbed{
			Title:       "🏓 Pong!",
			Description: fmt.Sprintf("Bot is responsive. Response time: %.2fms", float64(time.Since(startTime).Nanoseconds())/1000000),
			Color:       0x00ff00,
			Timestamp:   time.Now().Format(time.RFC3339),
		}
		if _, err := s.ChannelMessageSendEmbed(m.ChannelID, embed); err != nil {
			b.logger.WithDiscord().WithError(err).Error("Failed to send ping command response")
		}

	default:
		embed := &discordgo.MessageEmbed{
			Title:       "❓ Unknown Command",
			Description: fmt.Sprintf("Unknown command: `%s`\nUse `!osrs help` to see available commands.", command),
			Color:       0xffaa00,
			Timestamp:   time.Now().Format(time.RFC3339),
		}
		if _, err := s.ChannelMessageSendEmbed(m.ChannelID, embed); err != nil {
			b.logger.WithDiscord().WithError(err).Error("Failed to send unknown command response")
		}
	}

	b.logger.WithDiscord().WithField("user_id", m.Author.ID).WithFields(logrus.Fields{
		"command":         command,
		"message":         m.Content,
		"processing_time": time.Since(b.lastCommandTime),
	}).Info("Bot command completed")
}

// IsReady returns whether the bot is ready to send messages
func (b *Bot) IsReady() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.ready
}

// GetChannelID returns the configured channel ID
func (b *Bot) GetChannelID() string {
	return b.channelID
}
