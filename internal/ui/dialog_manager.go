package ui

import (
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// DialogKind 弹窗类别，用于按类别批量关闭（如失败类弹窗在状态恢复后自动关闭）。
type DialogKind int

const (
	// DialogKindInfo 信息弹窗。
	DialogKindInfo DialogKind = iota
	// DialogKindError 错误弹窗。
	DialogKindError
	// DialogKindAlert 告警弹窗（如连接中断等失败类提醒，随状态恢复自动关闭）。
	DialogKindAlert
	// DialogKindConfirm 确认弹窗。
	DialogKindConfirm
	// DialogKindForm 表单弹窗。
	DialogKindForm
)

// DialogManager 统一管理主窗口弹窗，解决弹窗堆叠与残留问题：
//   - 同一时刻最多只有一个弹窗：展示新弹窗前自动关闭旧弹窗；
//   - 失败/告警类弹窗（Error/Alert）可在代理恢复、切换节点、重新打开窗口等场景下调用
//     DismissFailure 主动关闭，避免残留失效弹窗。
//
// 所有方法均可在任意 goroutine 调用：内部通过 fyne.Do 串行到主线程执行。
type DialogManager struct {
	mu      sync.Mutex
	window  fyne.Window
	current dialog.Dialog
	kind    DialogKind
}

// NewDialogManager 创建弹窗管理器。window 可在后续通过 SetWindow 设置。
func NewDialogManager() *DialogManager {
	return &DialogManager{}
}

// SetWindow 设置弹窗所属窗口。
func (m *DialogManager) SetWindow(w fyne.Window) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.window = w
}

// Dismiss 关闭当前弹窗（如有）。
func (m *DialogManager) Dismiss() {
	fyne.Do(func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.dismissLocked()
	})
}

// DismissFailure 关闭当前失败/告警类弹窗（Error / Alert），
// 用于代理启动成功、切换节点、重新打开窗口等状态恢复场景，避免残留失效弹窗。
func (m *DialogManager) DismissFailure() {
	fyne.Do(func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.kind == DialogKindError || m.kind == DialogKindAlert {
			m.dismissLocked()
		}
	})
}

// dismissLocked 关闭当前弹窗（调用方须已持有 mu）。
func (m *DialogManager) dismissLocked() {
	if m.current != nil {
		m.current.Hide()
		m.current = nil
	}
}

// ShowInfo 显示信息弹窗。
func (m *DialogManager) ShowInfo(title, message string) {
	m.show(DialogKindInfo, func(w fyne.Window) dialog.Dialog {
		return dialog.NewInformation(title, message, w)
	})
}

// ShowError 显示错误弹窗。
func (m *DialogManager) ShowError(err error) {
	m.show(DialogKindError, func(w fyne.Window) dialog.Dialog {
		return dialog.NewError(err, w)
	})
}

// ShowAlert 显示告警弹窗（如连接中断等失败类提醒）。
func (m *DialogManager) ShowAlert(title, message string) {
	m.show(DialogKindAlert, func(w fyne.Window) dialog.Dialog {
		return dialog.NewInformation(title, message, w)
	})
}

// ShowConfirm 显示确认弹窗。onConfirm 在用户确认(true)或取消(false)时回调。
func (m *DialogManager) ShowConfirm(title, message string, onConfirm func(bool)) {
	m.show(DialogKindConfirm, func(w fyne.Window) dialog.Dialog {
		return dialog.NewConfirm(title, message, onConfirm, w)
	})
}

// ShowForm 显示表单弹窗（确认/取消）。callback 在用户确认(true)或取消(false)时回调。
func (m *DialogManager) ShowForm(title, confirmText, dismissText string, items []*widget.FormItem, callback func(bool)) {
	m.ShowFormSized(title, confirmText, dismissText, items, callback, fyne.Size{})
}

// ShowFormSized 显示表单弹窗并指定最小尺寸（size 为零值时使用默认尺寸）。
func (m *DialogManager) ShowFormSized(title, confirmText, dismissText string, items []*widget.FormItem, callback func(bool), size fyne.Size) {
	m.show(DialogKindForm, func(w fyne.Window) dialog.Dialog {
		dlg := dialog.NewForm(title, confirmText, dismissText, items, callback, w)
		if size.Width > 0 || size.Height > 0 {
			dlg.Resize(size)
		}
		return dlg
	})
}

// show 串行到主线程：先关闭旧弹窗，再创建并展示新弹窗，保证同一时刻只有一个弹窗。
func (m *DialogManager) show(kind DialogKind, build func(fyne.Window) dialog.Dialog) {
	fyne.Do(func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.window == nil {
			return
		}
		m.dismissLocked()
		dlg := build(m.window)
		m.current = dlg
		m.kind = kind
		dlg.Show()
	})
}
