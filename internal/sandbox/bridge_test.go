package sandbox

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireUV(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("uv"); err != nil {
		t.Skip("uv not available, skipping bridge test")
	}
}

func TestBridge_SimpleArithmetic(t *testing.T) {
	requireUV(t)

	b, err := NewBridge()
	require.NoError(t, err)
	defer b.Shutdown()

	result, err := b.RunScript("2 + 3", nil)
	require.NoError(t, err)
	assert.InDelta(t, float64(5), result, 0.001)
}

func TestBridge_PrimitiveCallback(t *testing.T) {
	requireUV(t)

	b, err := NewBridge()
	require.NoError(t, err)
	defer b.Shutdown()

	b.RegisterPrimitive("add", func(args []any, _ map[string]any) (any, error) {
		a := args[0].(float64)
		b := args[1].(float64)
		return a + b, nil
	})

	result, err := b.RunScript(`add(10, 20)`, []string{"add"})
	require.NoError(t, err)
	assert.InDelta(t, float64(30), result, 0.001)
}

func TestBridge_PrimitiveKwargs(t *testing.T) {
	requireUV(t)

	b, err := NewBridge()
	require.NoError(t, err)
	defer b.Shutdown()

	b.RegisterPrimitive("greet", func(_ []any, kwargs map[string]any) (any, error) {
		name, _ := kwargs["name"].(string)
		return "hello " + name, nil
	})

	result, err := b.RunScript(`greet(name="world")`, []string{"greet"})
	require.NoError(t, err)
	assert.Equal(t, "hello world", result)
}

func TestBridge_ScriptError(t *testing.T) {
	requireUV(t)

	b, err := NewBridge()
	require.NoError(t, err)
	defer b.Shutdown()

	_, err = b.RunScript(`x = 1 / 0`, nil)
	require.Error(t, err)
}

func TestBridge_UnknownPrimitive(t *testing.T) {
	requireUV(t)

	b, err := NewBridge()
	require.NoError(t, err)
	defer b.Shutdown()

	_, err = b.RunScript(`nonexistent()`, []string{"nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown primitive")
}

func TestBridge_Shutdown(t *testing.T) {
	requireUV(t)

	b, err := NewBridge()
	require.NoError(t, err)

	err = b.Shutdown()
	require.NoError(t, err)
}

func TestBridge_PrimitiveNames(t *testing.T) {
	requireUV(t)

	b, err := NewBridge()
	require.NoError(t, err)
	defer b.Shutdown()

	b.RegisterPrimitive("foo", func(_ []any, _ map[string]any) (any, error) { return true, nil })
	b.RegisterPrimitive("bar", func(_ []any, _ map[string]any) (any, error) { return true, nil })

	names := b.PrimitiveNames()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "foo")
	assert.Contains(t, names, "bar")
}

func TestBridge_TrueResult(t *testing.T) {
	requireUV(t)

	b, err := NewBridge()
	require.NoError(t, err)
	defer b.Shutdown()

	b.RegisterPrimitive("noop", func(_ []any, _ map[string]any) (any, error) {
		return true, nil
	})

	result, err := b.RunScript(`noop()`, []string{"noop"})
	require.NoError(t, err)
	assert.Equal(t, true, result)
}
