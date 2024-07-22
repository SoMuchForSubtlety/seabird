package widget

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"strings"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4-sourceview/pkg/gtksource/v5"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/getseabird/seabird/api"
	"github.com/getseabird/seabird/internal/util"
	"github.com/leaanthony/go-ansi-parser"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

type LogPage struct {
	*adw.NavigationPage
}

func NewLogPage(ctx context.Context, cluster *api.Cluster, pod *corev1.Pod, container string) *LogPage {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("view")
	p := LogPage{NavigationPage: adw.NewNavigationPage(box, container)}

	header := adw.NewHeaderBar()
	header.SetShowStartTitleButtons(false)
	header.AddCSSClass("flat")
	box.Append(header)

	buffer := gtksource.NewBuffer(nil)
	util.SetSourceColorScheme(buffer)
	view := gtksource.NewViewWithBuffer(buffer)
	view.SetEditable(false)
	view.SetWrapMode(gtk.WrapWord)
	view.SetShowLineNumbers(true)
	view.SetMonospace(true)

	scrolledWindow := gtk.NewScrolledWindow()
	scrolledWindow.SetChild(view)
	scrolledWindow.SetVExpand(true)
	box.Append(scrolledWindow)

	logCtx, cancel := context.WithCancel(ctx)
	p.ConnectDestroy(func() {
		cancel()
	})
	logs, err := podLogs(logCtx, cluster, pod, container)
	if err != nil {
		ShowErrorDialog(ctx, "Could not load logs", err)
	} else {
		scanner := bufio.NewScanner(logs)
		go func() {
			defer logs.Close()
			for scanner.Scan() {
				scanner.Err()
				content := scanner.Text()
				if text, err := ansi.Parse(content); err != nil {
					buffer.SetText(content)
				} else {
					for _, text := range text {
						var attr []string
						if text.FgCol != nil {
							attr = append(attr, fmt.Sprintf(`foreground="%s"`, text.FgCol.Hex))
						}
						if text.BgCol != nil {
							attr = append(attr, fmt.Sprintf(`background="%s"`, text.BgCol.Hex))
						}
						buffer.InsertMarkup(buffer.EndIter(), fmt.Sprintf(`<span %s>%s</span>`, strings.Join(attr, " "), html.EscapeString(text.Label)))
					}
				}
			}
			if scanner.Err() != nil && !errors.Is(err, context.Canceled) {
				ShowErrorDialog(ctx, "Could not scan for more logs", err)
			}
		}()
	}

	return &p
}

func podLogs(ctx context.Context, cluster *api.Cluster, pod *corev1.Pod, container string) (io.ReadCloser, error) {
	req := cluster.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: container, Follow: true, SinceSeconds: ptr.To[int64](600)})
	return req.Stream(ctx)
}
