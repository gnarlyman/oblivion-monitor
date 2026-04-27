package main

import (
	"fmt"
	"sync"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

const (
	windowWidth    = 320
	windowHeight   = 220
	sparklineWidth = 13
	historyDepth   = 60 // 60 samples = 1 minute at 1s polling
)

// metricRow is the per-metric mutable display state.
type metricRow struct {
	label   string
	unit    string
	history []float64
	current float64
	missing int // consecutive zero/NaN samples
}

type UI struct {
	cfg    Config
	cfgMu  sync.Mutex
	mw     *walk.MainWindow
	header *walk.TextLabel
	rows   map[string]*walk.TextLabel
	state  map[string]*metricRow
	tray   *walk.NotifyIcon

	pid        uint32
	procName   string
	configPath string
}

// metricKeys defines display order.
var metricKeys = []string{"vram_ded", "vram_shr", "ram", "page", "cpu"}

// NewUI builds the window. configPath is where to persist position.
func NewUI(cfg Config, configPath string) (*UI, error) {
	u := &UI{
		cfg:        cfg,
		state:      map[string]*metricRow{},
		rows:       map[string]*walk.TextLabel{},
		configPath: configPath,
	}
	u.state["vram_ded"] = &metricRow{label: "VRAM (ded)", unit: "MB"}
	u.state["vram_shr"] = &metricRow{label: "VRAM (shr)", unit: "MB"}
	u.state["ram"] = &metricRow{label: "RAM", unit: "MB"}
	u.state["page"] = &metricRow{label: "Page file", unit: "MB"}
	u.state["cpu"] = &metricRow{label: "CPU", unit: "%"}

	var headerLbl *walk.TextLabel
	rowLabels := map[string]**walk.TextLabel{}
	rowChildren := []Widget{}
	for _, k := range metricKeys {
		var lbl *walk.TextLabel
		rowLabels[k] = &lbl
		rowChildren = append(rowChildren, TextLabel{
			AssignTo: &lbl,
			Font:     Font{Family: "Consolas", PointSize: 10},
		})
	}

	if err := (MainWindow{
		AssignTo: &u.mw,
		Title:    "oblivion-monitor",
		Size:     Size{Width: windowWidth, Height: windowHeight},
		Layout: VBox{
			MarginsZero: false,
			Margins:     Margins{Left: 8, Top: 8, Right: 8, Bottom: 8},
		},
		Children: append([]Widget{
			TextLabel{
				AssignTo: &headerLbl,
				Font:     Font{Family: "Consolas", PointSize: 9, Bold: true},
			},
		}, rowChildren...),
	}).Create(); err != nil {
		return nil, err
	}

	u.header = headerLbl
	for _, k := range metricKeys {
		u.rows[k] = *rowLabels[k]
	}

	// Make borderless + always-on-top.
	// WS_POPUP = 0x80000000 overflows int32 as a constant, so we reinterpret
	// through uint32 at runtime.
	styleU := uint32(win.GetWindowLong(u.mw.Handle(), win.GWL_STYLE))
	styleU &^= win.WS_BORDER | win.WS_THICKFRAME | win.WS_MINIMIZEBOX | win.WS_MAXIMIZEBOX | win.WS_SYSMENU
	styleU |= win.WS_POPUP
	win.SetWindowLong(u.mw.Handle(), win.GWL_STYLE, int32(styleU))
	win.SetWindowPos(u.mw.Handle(), win.HWND_TOPMOST, 0, 0, 0, 0,
		win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOACTIVATE|win.SWP_FRAMECHANGED)

	// Compute initial position.
	x, y := u.computePosition()
	u.mw.SetBounds(walk.Rectangle{X: x, Y: y, Width: windowWidth, Height: windowHeight})

	// Click-anywhere drag.
	u.mw.MouseDown().Attach(func(_, _ int, _ walk.MouseButton) {
		win.ReleaseCapture()
		win.SendMessage(u.mw.Handle(), win.WM_NCLBUTTONDOWN, win.HTCAPTION, 0)
		// Persist position after drag.
		u.persistPosition()
	})

	// Tray icon.
	tray, err := walk.NewNotifyIcon(u.mw)
	if err != nil {
		return nil, err
	}
	icon, _ := walk.NewIconFromResourceId(2) // 2 = Windows default app icon
	if icon != nil {
		tray.SetIcon(icon)
	}
	tray.SetToolTip("oblivion-monitor")
	exitAction := walk.NewAction()
	exitAction.SetText("E&xit")
	exitAction.Triggered().Attach(func() { walk.App().Exit(0) })
	tray.ContextMenu().Actions().Add(exitAction)
	tray.SetVisible(true)
	u.tray = tray

	// Start hidden — visible only when PID is live.
	u.mw.SetVisible(false)
	return u, nil
}

func (u *UI) computePosition() (int, int) {
	if u.cfg.WindowX != nil && u.cfg.WindowY != nil {
		return *u.cfg.WindowX, *u.cfg.WindowY
	}
	screenW := int(win.GetSystemMetrics(win.SM_CXSCREEN))
	return screenW - windowWidth - 20, 20
}

func (u *UI) persistPosition() {
	bounds := u.mw.Bounds()
	u.cfgMu.Lock()
	defer u.cfgMu.Unlock()
	x, y := bounds.X, bounds.Y
	u.cfg.WindowX = &x
	u.cfg.WindowY = &y
	_ = SaveConfig(u.configPath, u.cfg)
}

// OnPIDFound makes the window visible and tracks the active PID.
func (u *UI) OnPIDFound(pid uint32, procName string) {
	u.mw.Synchronize(func() {
		u.pid = pid
		u.procName = procName
		// Reset history.
		for _, r := range u.state {
			r.history = r.history[:0]
			r.missing = 0
		}
		u.mw.SetVisible(true)
	})
}

// OnPIDLost hides the window.
func (u *UI) OnPIDLost() {
	u.mw.Synchronize(func() {
		u.pid = 0
		u.mw.SetVisible(false)
	})
}

// OnSample updates the display.
func (u *UI) OnSample(s Sample) {
	values := map[string]float64{
		"vram_ded": s.VRAMDedicatedMB,
		"vram_shr": s.VRAMSharedMB,
		"ram":      s.RAMPrivateMB,
		"page":     s.PageFileMB,
		"cpu":      s.CPUPct,
	}
	u.mw.Synchronize(func() {
		u.header.SetText(fmt.Sprintf("%s (PID %d) — %02d:%02d:%02d",
			u.procName, u.pid,
			s.UptimeSec/3600, (s.UptimeSec%3600)/60, s.UptimeSec%60))
		for _, k := range metricKeys {
			r := u.state[k]
			v := values[k]
			r.current = v
			if v == 0 {
				r.missing++
			} else {
				r.missing = 0
			}
			r.history = append(r.history, v)
			if len(r.history) > historyDepth {
				r.history = r.history[len(r.history)-historyDepth:]
			}
			displayVal := fmt.Sprintf("%.0f", v)
			if r.missing > 5 {
				displayVal = "--"
			}
			line := fmt.Sprintf("%-10s %6s %-3s   %s",
				r.label, displayVal, r.unit,
				Sparkline(r.history, sparklineWidth))
			u.rows[k].SetText(line)
		}
	})
}

// Run blocks on the message loop.
func (u *UI) Run() {
	u.mw.Run()
}
