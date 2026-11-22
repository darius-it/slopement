package kit

import "strings"

// Emoji is an emoji with its name and the words it is also found by.
type Emoji struct {
	Char, Name string
	// Words are extra search words ("joy tears laugh"), lowercase.
	Words string
}

// EmojiCategory is a section of the emoji picker.
type EmojiCategory struct {
	Name   string
	Emojis []Emoji
}

func em(pairs ...string) []Emoji {
	out := make([]Emoji, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, Emoji{Char: pairs[i], Words: pairs[i+1]})
	}
	return out
}

// curatedCategories are the commonly used emoji of each group with the words
// people look for them by; the full set (emojiset.go) takes these words.
var curatedCategories = []EmojiCategory{
	{"Smileys", em(
		"😀", "grinning", "😃", "smiley", "😄", "smile", "😁", "grin", "😆", "laughing", "😅", "sweat smile",
		"😂", "joy tears laugh", "🤣", "rofl rolling", "🥲", "smiling tear", "😊", "blush", "😇", "innocent halo",
		"🙂", "slightly smiling", "🙃", "upside down", "😉", "wink", "😌", "relieved", "😍", "heart eyes",
		"🥰", "smiling hearts", "😘", "kiss", "😋", "yum", "😛", "tongue", "😜", "winking tongue", "🤪", "zany",
		"🤨", "raised eyebrow", "🧐", "monocle", "🤓", "nerd", "😎", "sunglasses cool", "🥳", "party",
		"😏", "smirk", "😒", "unamused", "😞", "disappointed", "😔", "pensive", "😟", "worried", "😕", "confused",
		"🙁", "frowning", "😣", "persevere", "😖", "confounded", "😫", "tired", "😩", "weary", "🥺", "pleading",
		"😢", "cry", "😭", "sob", "😤", "triumph huff", "😠", "angry", "😡", "rage", "🤯", "exploding head",
		"😳", "flushed", "🥵", "hot", "🥶", "cold", "😱", "scream", "😨", "fearful", "🤔", "thinking",
		"🫠", "melting", "🤭", "hand over mouth", "🤫", "shush", "😶", "no mouth", "😐", "neutral", "😑", "expressionless",
		"😬", "grimace", "🙄", "eye roll", "😴", "sleeping", "🤤", "drool", "😮", "open mouth", "🤐", "zipper",
		"🤢", "nauseated", "🤮", "vomit", "🤧", "sneeze", "😷", "mask", "🤠", "cowboy", "🤡", "clown",
		"👻", "ghost", "💀", "skull dead", "👽", "alien", "🤖", "robot", "💩", "poop",
	)},
	{"People", em(
		"👋", "wave hello", "🤚", "raised back hand", "✋", "hand stop", "🖖", "vulcan", "👌", "ok",
		"🤌", "pinched fingers", "✌️", "victory peace", "🤞", "crossed fingers luck", "🫶", "heart hands",
		"🤟", "love you", "🤘", "rock", "🤙", "call me", "👈", "point left", "👉", "point right", "👆", "point up",
		"👇", "point down", "👍", "thumbs up yes", "👎", "thumbs down no", "✊", "fist", "👊", "punch",
		"👏", "clap", "🙌", "raised hands", "👐", "open hands", "🤝", "handshake", "🙏", "pray thanks please",
		"💪", "muscle strong", "🫡", "salute", "👀", "eyes look", "🧠", "brain", "🫂", "hug",
		"🤷", "shrug", "🤦", "facepalm", "🙋", "raising hand", "🙇", "bow",
	)},
	{"Nature", em(
		"🐶", "dog", "🐱", "cat", "🐭", "mouse", "🐰", "rabbit bunny", "🦊", "fox", "🐻", "bear",
		"🐼", "panda", "🐨", "koala", "🐯", "tiger", "🦁", "lion", "🐮", "cow", "🐷", "pig",
		"🐸", "frog", "🐵", "monkey", "🐔", "chicken", "🐧", "penguin", "🐦", "bird", "🦆", "duck",
		"🦉", "owl", "🐝", "bee", "🦋", "butterfly", "🐢", "turtle", "🐍", "snake", "🐙", "octopus",
		"🦈", "shark", "🐳", "whale", "🌵", "cactus", "🌲", "tree", "🍀", "clover luck", "🌸", "blossom",
		"🌻", "sunflower", "🌈", "rainbow", "☀️", "sun", "🌙", "moon", "⭐", "star", "⚡", "lightning zap",
		"🔥", "fire lit", "❄️", "snowflake", "🌊", "wave water",
	)},
	{"Food", em(
		"🍏", "apple green", "🍎", "apple", "🍋", "lemon", "🍌", "banana", "🍉", "watermelon", "🍇", "grapes",
		"🍓", "strawberry", "🍑", "peach", "🥑", "avocado", "🌶️", "pepper hot", "🥐", "croissant", "🍞", "bread",
		"🧀", "cheese", "🍳", "egg cooking", "🥓", "bacon", "🍔", "burger", "🍟", "fries", "🍕", "pizza",
		"🌮", "taco", "🍜", "ramen noodles", "🍣", "sushi", "🍙", "rice ball", "🍩", "donut", "🍪", "cookie",
		"🎂", "cake birthday", "🍫", "chocolate", "🍿", "popcorn", "☕", "coffee", "🍵", "tea", "🧋", "bubble tea",
		"🍺", "beer", "🍷", "wine", "🥂", "cheers", "🧃", "juice",
	)},
	{"Activities", em(
		"⚽", "soccer football", "🏀", "basketball", "🏈", "american football", "🎾", "tennis", "🏐", "volleyball",
		"🎱", "8ball pool", "🏓", "ping pong", "🎯", "target bullseye", "🎮", "video game", "🕹️", "joystick",
		"🎲", "dice", "♟️", "chess", "🎨", "art palette", "🎬", "movie clapper", "🎤", "microphone karaoke",
		"🎧", "headphones music", "🎸", "guitar", "🎹", "piano", "🏆", "trophy win", "🥇", "gold medal",
		"🎉", "tada party", "🎊", "confetti", "🎁", "gift present", "🎈", "balloon",
	)},
	{"Travel", em(
		"🚗", "car", "🚕", "taxi", "🚌", "bus", "🚲", "bike", "🛴", "scooter", "✈️", "airplane", "🚀", "rocket",
		"🛸", "ufo", "🚂", "train", "⛵", "sailboat", "🏠", "house home", "🏢", "office", "🏖️", "beach",
		"🏔️", "mountain", "🗺️", "map", "🧭", "compass", "🌍", "earth world", "🗼", "tower", "⛺", "tent camping",
	)},
	{"Objects", em(
		"💡", "bulb idea", "📱", "phone mobile", "💻", "laptop computer", "⌨️", "keyboard", "🖥️", "desktop",
		"📷", "camera", "🔋", "battery", "🔌", "plug", "💸", "money flying", "💰", "money bag", "🔑", "key",
		"🔒", "lock", "🔨", "hammer", "🛠️", "tools", "⚙️", "gear", "📦", "package box", "✉️", "envelope mail",
		"📌", "pin", "📎", "paperclip", "✂️", "scissors", "📝", "memo note", "📅", "calendar", "📚", "books",
		"🔍", "search magnifier", "🧪", "test tube", "💊", "pill", "⏰", "alarm clock", "🕯️", "candle",
	)},
	{"Symbols", em(
		"❤️", "heart red love", "🧡", "orange heart", "💛", "yellow heart", "💚", "green heart", "💙", "blue heart",
		"💜", "purple heart", "🖤", "black heart", "🤍", "white heart", "💔", "broken heart", "💯", "hundred",
		"✨", "sparkles", "💫", "dizzy", "💥", "boom", "💬", "speech", "💤", "zzz sleep", "✅", "check done",
		"☑️", "ballot check", "❌", "cross no", "❓", "question", "❗", "exclamation", "⚠️", "warning",
		"🚫", "prohibited", "♻️", "recycle", "🔴", "red circle", "🟣", "purple circle", "➕", "plus", "➖", "minus",
		"🆗", "ok button", "🆒", "cool", "🆕", "new", "🔝", "top",
	)},
}

// DefaultRecentEmoji fill the recently used row before the user has any.
var DefaultRecentEmoji = []string{"👍", "❤️", "😂", "🎉", "🔥", "🙏", "👀", "✨", "😭", "🥲", "💜", "✅"}

// EmojiCategories is the picker's set: every emoji of Unicode's emoji list
// (without the skin-tone variants), in categories as Discord's, built once
// from the full data (emojiset.go) with the curated words above kept as extra
// ways to find them.
var EmojiCategories []EmojiCategory

var emojiNames map[string]string

func init() {
	EmojiCategories = buildEmojiCategories()
	emojiNames = map[string]string{}
	for _, cat := range EmojiCategories {
		for _, e := range cat.Emojis {
			emojiNames[e.Char] = e.Name
		}
	}
}

// EmojiName returns the name of e, for labels ("face with tears of joy").
func EmojiName(e string) string {
	if n, ok := emojiNames[e]; ok {
		return n
	}
	return emojiNames[strings.ReplaceAll(e, "️", "")+"️"]
}

// emojiShortcode is a name as a ":shortcode:" word ("face_with_tears_of_joy").
func emojiShortcode(name string) string { return strings.ReplaceAll(name, " ", "_") }

// SearchEmoji returns the emoji whose name or keywords contain every word of
// query.
func SearchEmoji(query string) []Emoji {
	words := strings.Fields(strings.ToLower(strings.ReplaceAll(query, "_", " ")))
	var out []Emoji
	for _, cat := range EmojiCategories {
	next:
		for _, e := range cat.Emojis {
			for _, w := range words {
				if !strings.Contains(e.Name, w) && !strings.Contains(e.Words, w) {
					continue next
				}
			}
			out = append(out, e)
		}
	}
	return out
}

// PushRecent moves item to the front of list, keeping at most max.
func PushRecent(list []string, item string, max int) []string {
	out := make([]string, 0, max)
	out = append(out, item)
	for _, x := range list {
		if x != item && len(out) < max {
			out = append(out, x)
		}
	}
	return out
}
