# 内存调优指南

## 快速诊断
```bash
go test -bench=. -memprofile=mem.out ./...
go tool pprof -http=:8080 mem.out
```

## 常见优化
- 使用 sync.Pool 复用临时对象
- 避免在热路径分配大对象
- 预分配 slice 容量
