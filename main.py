import discord
from discord.ext import commands, tasks
import asyncio
import aiohttp
from datetime import datetime
import json
import os
from flask import Flask, request, jsonify
import threading

# Bot configuration
BOT_TOKEN = os.getenv('DISCORD_BOT_TOKEN')
CHANNEL_ID = int(os.getenv('DISCORD_CHANNEL_ID', '0'))  # Channel for scheduled tasks
ALERTS_CHANNEL_ID = int(os.getenv('DISCORD_ALERTS_CHANNEL_ID', '0'))  # Channel for alerts

# Discord bot setup
intents = discord.Intents.default()
intents.message_content = True
bot = commands.Bot(command_prefix='!', intents=intents)

# Flask app for webhook receiver
app = Flask(__name__)

class DiscordBot:
    def __init__(self, bot_instance):
        self.bot = bot_instance

    async def send_scheduled_message(self, channel_id, message):
        """Send a scheduled task result to Discord"""
        try:
            channel = self.bot.get_channel(channel_id)
            if channel:
                embed = discord.Embed(
                    title="📊 Scheduled Task Result",
                    description=message,
                    color=0x00ff00,
                    timestamp=datetime.now()
                )
                await channel.send(embed=embed)
                print(f"Scheduled message sent to {channel.name}")
            else:
                print(f"Channel {channel_id} not found")
        except Exception as e:
            print(f"Error sending scheduled message: {e}")

    async def send_prometheus_alert(self, channel_id, alert_data):
        """Send Prometheus alert to Discord"""
        try:
            channel = self.bot.get_channel(channel_id)
            if not channel:
                print(f"Alert channel {channel_id} not found")
                return

            # Parse Prometheus alert
            alerts = alert_data.get('alerts', [])

            for alert in alerts:
                status = alert.get('status', 'unknown')
                alert_name = alert.get('labels', {}).get('alertname', 'Unknown Alert')
                instance = alert.get('labels', {}).get('instance', 'Unknown Instance')
                severity = alert.get('labels', {}).get('severity', 'unknown')
                description = alert.get('annotations', {}).get('description', 'No description available')
                summary = alert.get('annotations', {}).get('summary', alert_name)

                # Set color based on status and severity
                color = 0xff0000  # Red for firing
                if status == 'resolved':
                    color = 0x00ff00  # Green for resolved
                elif severity == 'warning':
                    color = 0xffa500  # Orange for warnings

                # Create embed
                embed = discord.Embed(
                    title=f"🚨 {alert_name}" if status == 'firing' else f"✅ {alert_name} - Resolved",
                    description=summary,
                    color=color,
                    timestamp=datetime.now()
                )

                embed.add_field(name="Status", value=status.upper(), inline=True)
                embed.add_field(name="Severity", value=severity.upper(), inline=True)
                embed.add_field(name="Instance", value=instance, inline=True)
                embed.add_field(name="Description", value=description, inline=False)

                # Add additional labels as fields
                labels = alert.get('labels', {})
                if labels:
                    label_text = "\n".join([f"**{k}:** {v}" for k, v in labels.items()
                                          if k not in ['alertname', 'instance', 'severity']])
                    if label_text:
                        embed.add_field(name="Labels", value=label_text[:1024], inline=False)

                await channel.send(embed=embed)
                print(f"Alert sent: {alert_name} ({status})")

        except Exception as e:
            print(f"Error sending Prometheus alert: {e}")

# Initialize bot instance
discord_bot = DiscordBot(bot)

# Bot events
@bot.event
async def on_ready():
    print(f'{bot.user} has connected to Discord!')
    print(f'Bot is in {len(bot.guilds)} guilds')

    # Start scheduled task
    if not scheduled_task.is_running():
        scheduled_task.start()

    print("Bot is ready and scheduled tasks are running!")

@bot.event
async def on_command_error(ctx, error):
    if isinstance(error, commands.CommandNotFound):
        await ctx.send("Command not found. Use `!help` to see available commands.")
    else:
        print(f"Error: {error}")
        await ctx.send(f"An error occurred: {error}")

# Bot commands
@bot.command(name='ping')
async def ping(ctx):
    """Test if bot is responsive"""
    latency = round(bot.latency * 1000)
    await ctx.send(f'Pong! Latency: {latency}ms')

@bot.command(name='status')
async def status(ctx):
    """Show bot status"""
    embed = discord.Embed(
        title="🤖 Bot Status",
        color=0x00ff00,
        timestamp=datetime.now()
    )
    embed.add_field(name="Guilds", value=len(bot.guilds), inline=True)
    embed.add_field(name="Latency", value=f"{round(bot.latency * 1000)}ms", inline=True)
    embed.add_field(name="Scheduled Task", value="Running" if scheduled_task.is_running() else "Stopped", inline=True)
    await ctx.send(embed=embed)

@bot.command(name='test_alert')
async def test_alert(ctx):
    """Send a test alert"""
    test_alert_data = {
        "alerts": [{
            "status": "firing",
            "labels": {
                "alertname": "Test Alert",
                "instance": "test-instance:9090",
                "severity": "warning"
            },
            "annotations": {
                "summary": "This is a test alert",
                "description": "Test alert triggered manually from Discord command"
            }
        }]
    }

    await discord_bot.send_prometheus_alert(ALERTS_CHANNEL_ID or ctx.channel.id, test_alert_data)
    await ctx.send("Test alert sent!")

# Scheduled task example
@tasks.loop(minutes=30)  # Run every 30 minutes
async def scheduled_task():
    """Example scheduled task - customize this for your needs"""
    try:
        # Example: Check system status, API health, etc.
        current_time = datetime.now().strftime("%Y-%m-%d %H:%M:%S")

        # Simulate some task result
        task_result = f"System check completed at {current_time}\n"
        task_result += "✅ All services are running normally\n"
        task_result += "📊 Memory usage: 65%\n"
        task_result += "🔧 Last backup: 2 hours ago"

        if CHANNEL_ID:
            await discord_bot.send_scheduled_message(CHANNEL_ID, task_result)
        else:
            print(f"Scheduled task completed: {task_result}")

    except Exception as e:
        print(f"Error in scheduled task: {e}")

@scheduled_task.before_loop
async def before_scheduled_task():
    await bot.wait_until_ready()

# Flask webhook receiver for Prometheus alerts
@app.route('/webhook/prometheus', methods=['POST'])
def prometheus_webhook():
    """Receive Prometheus alerts via webhook"""
    try:
        alert_data = request.get_json()

        if not alert_data:
            return jsonify({"error": "No JSON data received"}), 400

        print(f"Received Prometheus alert: {json.dumps(alert_data, indent=2)}")

        # Send alert to Discord asynchronously
        asyncio.run_coroutine_threadsafe(
            discord_bot.send_prometheus_alert(ALERTS_CHANNEL_ID, alert_data),
            bot.loop
        )

        return jsonify({"status": "success", "message": "Alert received"}), 200

    except Exception as e:
        print(f"Error processing webhook: {e}")
        return jsonify({"error": str(e)}), 500

@app.route('/health', methods=['GET'])
def health_check():
    """Health check endpoint"""
    return jsonify({
        "status": "healthy",
        "bot_status": "connected" if bot.is_ready() else "disconnected",
        "timestamp": datetime.now().isoformat()
    }), 200

def run_flask():
    """Run Flask app in a separate thread"""
    app.run(host='0.0.0.0', port=5000, debug=False, use_reloader=False)

def main():
    """Main function to start both Discord bot and Flask webhook receiver"""
    if not BOT_TOKEN:
        print("ERROR: DISCORD_BOT_TOKEN environment variable not set!")
        return

    if not CHANNEL_ID:
        print("WARNING: DISCORD_CHANNEL_ID not set - scheduled messages won't be sent")

    if not ALERTS_CHANNEL_ID:
        print("WARNING: DISCORD_ALERTS_CHANNEL_ID not set - alerts will be sent to the same channel as scheduled tasks")

    # Start Flask app in background thread
    flask_thread = threading.Thread(target=run_flask, daemon=True)
    flask_thread.start()
    print("Flask webhook server started on port 5000")

    # Start Discord bot
    print("Starting Discord bot...")
    bot.run(BOT_TOKEN)

if __name__ == "__main__":
    main()