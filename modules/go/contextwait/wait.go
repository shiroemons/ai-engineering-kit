// Package contextwait は共有状態を持たず、キャンセル可能な待機を提供する。
package contextwait

import (
	"context"
	"errors"
	"time"
)

// ErrNilContext は Wait に nil Context を渡した場合に返す。
var ErrNilContext = errors.New("contextwait: context must not be nil")

// Wait は d の経過またはキャンセルを待つ。d が0以下なら待機しない。
// 最後の確認で観測したキャンセルは、タイマーの完了より優先する。
// キャンセル時は context.Cause(ctx) ではなく ctx.Err() を返す。
func Wait(ctx context.Context, d time.Duration) error {
	if ctx == nil {
		return ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ctx.Err()
	}
}
