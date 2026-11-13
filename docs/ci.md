# GitHub Actions：Foundation 认证与排错

## PR #6 的实际失败

运行 [43415282301](https://github.com/0xunknownz/saas-tic-tools/actions/runs/43415282301) 中，api / security 在空 FOUNDATION_READ_TOKEN 的显式检查处退出；frontend 安装 @0xunknownz/foundation-platform-ui@0.2.1 返回 ERR_PNPM_FETCH_401。只读查询仓库 Actions secrets 返回空列表。

但 Foundation 源码仓库 [0xunknownz/saas.starter](https://github.com/0xunknownz/saas.starter) 当前为公开仓库，v0.2.1 标签已通过关闭凭据助手的 git ls-remote 匿名验证。因此 Go 构建不再因没有 PAT 而提前失败。私有包和源码仓库的访问要求必须分开判断。

## 工作流现在如何认证

- api / security：不配置 secret 时匿名读取公开 Foundation；配置后使用 FOUNDATION_READ_TOKEN 访问需要认证的源码。
- frontend：优先使用 FOUNDATION_READ_TOKEN，否则使用仅有 contents:read / packages:read 的 GITHUB_TOKEN。
- npm registry 地址可以留在项目 .npmrc；认证配置通过 RUNNER_TEMP 下的可信用户配置注入。配置文件只含 NODE_AUTH_TOKEN 的环境变量引用，不写入实际令牌值。pnpm 10 不展开项目级认证字段，因此已删除无效的项目级字段。
- main 分支 push 和 PR 触发 CI；同一 PR 的新提交取消旧运行，避免 feature 分支 push / PR 双重执行。保留 workflow_dispatch，合并到默认分支后可手动触发。

## 如包下载仍返回 401 / 403

先核对失败步骤及包名。如果 GITHUB_TOKEN 已可以下载依赖，就无需新增 PAT。源代码公开不代表对应 npm 包自动向其他仓库的工作流授权。

优先方案是在每个 Foundation 包的 Package settings → Manage Actions access 中，把 0xunknownz/saas-tic-tools 加入并授予 Read。检查依赖中的 @0xunknownz/foundation-client、@0xunknownz/foundation-platform-ui、@0xunknownz/foundation-tenant-ui。包设置由包管理员管理，仓库权限并不自动等同于包权限。

不能授予 Actions 访问时，使用下面的 repository secret 方案。

## 配置 FOUNDATION_READ_TOKEN

1. 用一个本身能够读取上述三个包的 GitHub 账号创建令牌。打开 [Personal access tokens (classic)](https://github.com/settings/tokens)，选择 Generate new token → Generate new token (classic)。设置名称（例如 saas-tic-tools-ci）和符合团队要求的有效期。
2. 当前源码公开，因此只需勾选 read:packages。不需要 write:packages、delete:packages 或 workflow。GitHub Packages 的 npm 个人令牌认证要求 classic PAT；不要用 fine-grained PAT 代替。若将来 Foundation 源码改成私有，读取私有 Git 源码还需 repo scope，并确保令牌所属账号有源码访问权；classic repo scope 较宽，应使用权限受限的专用账号。
3. 打开本项目 [Actions secrets 设置](https://github.com/0xunknownz/saas-tic-tools/settings/secrets/actions)，进入 Settings → Secrets and variables → Actions，点击 New repository secret。
4. Name 必须为 FOUNDATION_READ_TOKEN；将令牌填入 Secret 后保存。这是 repository secret，不是 Actions variable，也不放在未被 job 引用的 environment 中。不要把令牌写入仓库或聊天。
5. 回到 PR #6 的最新失败运行，选择 Re-run all jobs。若此前运行属于旧提交，则重新运行最新提交的 CI；旧提交不会包含工作流修复。令牌已过期、被撤销或令牌所属账号失去包权限时，需要更新同名 secret。

可选：也可以在自己的终端执行下列命令，由 gh 的交互提示输入令牌，不要把令牌放在命令行参数中：

```bash
gh secret set FOUNDATION_READ_TOKEN --repo 0xunknownz/saas-tic-tools
```

## 如何判断已修复

api / security 应进入实际 Build / Migrate / Test 步骤；frontend 应通过 pnpm install --frozen-lockfile，再执行 build、typecheck、test。通过认证后若出现编译或测试错误，应按新日志继续修复，不能将其混同为权限问题，也不能跳过检查来制造成功。

工作流本身使用 actionlint .github/workflows/ci.yml 验证。真实包权限和 Linux CI 结果以 PR 最新提交的 Actions 为准，本地已有依赖的构建不能证明新的认证方式可用。

官方依据：[npm registry authentication](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-npm-registry)、[repository Actions secrets](https://docs.github.com/en/actions/how-tos/write-workflows/choose-what-workflows-do/use-secrets)。
