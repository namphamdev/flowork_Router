// RTK Token Saver (router-side adapter).

package router

import (
	"github.com/flowork-os/flowork_Router/internal/rtk"
)

const rtkToolResultCap = 4000

// compressMessagesRTK returns a copy of msgs with tool-result content
// compressed and the total bytes saved across the conversation.
func compressMessagesRTK(msgs []OpenAIMessage) ([]OpenAIMessage, int) {
	out := make([]OpenAIMessage, len(msgs))
	copy(out, msgs)
	saved := 0
	for i := range out {
		if out[i].Role != "tool" {
			continue
		}
		c, n := rtk.Compress(out[i].Content, rtkToolResultCap)
		if n > 0 {
			out[i].Content = c
			saved += n
		}
	}
	return out, saved
}
