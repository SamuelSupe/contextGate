# Contributing / 参与贡献

Small, focused changes are welcome. Start an issue for a new adapter or a change to the authorization model so its boundary can be agreed first.

## Development

Use Go 1.26, a C/C++ toolchain, Node.js 24 and Docker. SQLite and DuckDB require CGO. Build the embedded UI before running Go tests:

```sh
make ui
go test ./...
go test -race ./...
node --test web/test/query.test.mjs
make build
```

The database matrix provisions isolated containers. Follow [validation](docs/validation.md) to create its `mcpdbhub-dev` container and network, then run `python3 scripts/matrix.py <product>`. Do not point fixture scripts at production data. Database compatibility claims need an independent passing product/version result; protocol compatibility alone is insufficient.

Keep queries read-only, apply authorization to metadata as well as data, preserve native types and parameter precision, and cancel work when access is revoked. Add a regression test when it protects meaningful behavior or a security boundary. Test rendered UI changes in local Chrome; do not add a browser automation framework for a small UI fix.

Before opening a pull request, remove credentials and local fixtures, explain the observable change, and report what you actually tested. Source and UI strings use English; maintain both README languages when user-facing instructions change.

## 中文

欢迎小而聚焦的改进。新数据库适配器或授权模型变更请先通过 Issue 讨论边界。使用上述命令构建内嵌前端并运行测试；Linux 后端与数据库默认在 OrbStack/Docker 中验证，前端通过本机 Chrome 操作检查。

测试数据只能写入隔离实例，不能把协议兼容当作产品实测。保持查询只读、元数据授权、无损参数及撤销取消语义；针对真实行为添加必要的回归测试。PR 应说明用户可观察的变化、实际检查结果及未验证范围，移除凭证与本地配置，并同步维护中英文使用说明。
