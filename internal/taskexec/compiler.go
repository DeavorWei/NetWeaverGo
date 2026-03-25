package taskexec

import (
	"context"
	"fmt"
	"sync"
)

// PlanCompiler 计划编译器接口 - 将任务定义编译为执行计划
type PlanCompiler interface {
	// Compile 将任务定义编译为执行计划
	Compile(ctx context.Context, def *TaskDefinition) (*ExecutionPlan, error)
	// Supports 返回是否支持该类型的任务
	Supports(kind string) bool
}

// CompilerRegistry 编译器注册表
type CompilerRegistry struct {
	compilers map[string]PlanCompiler
	mu        sync.RWMutex
}

// NewCompilerRegistry 创建编译器注册表
func NewCompilerRegistry() *CompilerRegistry {
	return &CompilerRegistry{
		compilers: make(map[string]PlanCompiler),
	}
}

// Register 注册编译器
func (r *CompilerRegistry) Register(kind string, compiler PlanCompiler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.compilers[kind] = compiler
}

// Get 获取编译器
func (r *CompilerRegistry) Get(kind string) (PlanCompiler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	compiler, ok := r.compilers[kind]
	return compiler, ok
}

// Compile 编译任务定义
func (r *CompilerRegistry) Compile(ctx context.Context, def *TaskDefinition) (*ExecutionPlan, error) {
	r.mu.RLock()
	compiler, ok := r.compilers[def.Kind]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("未找到任务类型的编译器: %s", def.Kind)
	}
	return compiler.Compile(ctx, def)
}
