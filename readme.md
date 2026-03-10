
# StarSleep 星眠

基于 Btrfs 与 OverlayFS 的声明式 Arch Linux 系统镜像分层构建工具。

StarSleep 通过 YAML 配置文件定义系统的分层结构，使用 OverlayFS 逐层叠加变更，再通过 reflink 展平合并到 Btrfs 子卷，最终生成不可变的系统快照。快照可直接通过 systemd-boot 引导启动。

灵感来源于 [arkdep](https://github.com/arkanelinux/arkdep)。

> **注意**: 项目中所有 `.go` 和 `.md` 文件均由 AI 生成，这意味着工具可能存在未知缺陷，请谨慎用于生产环境。

## 特性

- **声明式配置** — 通过 YAML 文件定义软件包、服务和层级顺序，构建结果可复现
- **分层构建** — 支持 `pacstrap`、`pacman`、`paru`、`enable_service`、`copy_files`、`chroot-cmd`、`chroot-pacman` 七种 helper，按层叠加
- **OverlayFS + Btrfs reflink** — 每层的 diff 持久存储在独立目录，展平合并时利用 reflink 实现零拷贝，低磁盘磨损
- **不可变快照** — 构建产物为 Btrfs 只读快照，支持回滚
- **systemd-boot 集成** — `flatten` 命令自动将内核和 initramfs 部署到 ESP 并生成引导条目
- **动态维护模式** — `maintain` 命令直接在运行中的系统上同步配置，清理多余包和孤立依赖
- **一致性校验** — `verify` 命令通过 rsync 对比展平目录与 OverlayFS 合并视图，确保展平结果正确
- **继承列表** — 通过 `inherit.list` 从宿主系统继承指定文件/目录到快照
- **i18n** — 支持中文/英文界面 (`--lang zh|en`)

## 前置条件

- Arch Linux
- 根文件系统位于 **Btrfs** 分区
- 使用 **systemd-boot** 引导
- 以下工具已安装:

  ```
  pacstrap  pacman  arch-chroot  mount  umount
  btrfs  getfattr  setfattr  rsync
  ```
- 内核已加载 `overlay` 模块

## 安装

```bash
git clone <repo-url> && cd starsleep-go
go build -o starsleep .
sudo install -m 755 starsleep /usr/local/bin/
```

## 快速开始

### 1. 初始化工作环境

```bash
sudo starsleep init
```

`init` **不会创建任何默认配置文件**，你需要自行编写配置。可以参考以下方案：

- **方案 A**: 直接在默认配置目录 `/starsleep/config/layers/` 下创建 YAML 文件（参见下方「编写配置」）
- **方案 B**: 在用户目录下维护配置（如 `~/starsleep-config/`），构建时通过 `-cp` 复制或 `-c` 直接引用：
  ```bash
  # 将用户配置复制到默认路径后构建
  sudo starsleep build -cp ~/starsleep-config

  # 或直接引用用户配置目录（不复制）
  sudo starsleep build -c ~/starsleep-config
  ```

`init` 将在 `/starsleep` 下创建以下目录结构：

```
/starsleep/
├── layers/            # 各层 diff 数据 (OverlayFS upper)
├── snapshots/         # 构建产物快照 (Btrfs subvolume)
├── shared/            # 跨构建持久化数据
│   ├── home/
│   └── pacman-cache/  # 包缓存共享
├── config/            # 配置文件
│   ├── layers/        # 层定义 (YAML)
│   ├── files/         # 叠加文件源目录 (copy_files 用)
│   └── inherit.list   # 继承列表
├── work/              # 构建工作区 (临时)
│   ├── flat/          # 展平子卷 (Btrfs)
│   ├── merged/        # OverlayFS 合并挂载点
│   └── ovl_work/      # OverlayFS 工作目录
└── logs/              # 构建日志
```

### 2. 编写配置

在 `/starsleep/config/layers/` 下按文件名排序创建 YAML 配置文件，每个文件定义一个构建层：

```yaml
# 00-base.yaml — 基础系统
name: base
helper: pacstrap
packages:
  - base
  - linux
  - linux-firmware
```

```yaml
# 01-desktop.yaml — 桌面环境
name: desktop
helper: pacman
packages:
  - plasma-desktop
  - sddm
  - firefox
```

```yaml
# 02-aur.yaml — AUR 软件包
name: aur-packages
helper: paru
packages:
  - paru
  - visual-studio-code-bin
```

```yaml
# 03-services.yaml — 启用服务
name: services
helper: enable_service
services:
  - NetworkManager
  - sddm
```

```yaml
# 04-overlay.yaml — 叠加配置文件
name: overlay-files
helper: copy_files
files:
  - src: etc/locale.conf        # 相对于 config/files/ 目录
    dst: /etc/locale.conf        # 目标系统中的路径
  - src: etc/pacman.d
    dst: /etc/pacman.d
```

```yaml
# 05-chroot-setup.yaml — 在 chroot 中执行命令
name: chroot-setup
helper: chroot-cmd
env:
  - key: LANG
    value: "zh_CN.UTF-8"          # 固定值
  - key: HOME
    host_key: HOME                  # 继承主机环境变量
commands:
  - locale-gen
  - systemd-machine-id-setup
```

```yaml
# 06-chroot-kernel.yaml — 通过 chroot 安装内核
name: chroot-kernel
helper: chroot-pacman
env:
  - key: LANG
    value: "C"
packages:
  - linux
  - linux-firmware
```

可选创建 `/starsleep/config/inherit.list`，从宿主系统继承文件到快照：

```
# 每行一个路径，支持注释
/etc/fstab
/etc/locale.conf
/etc/hostname
/etc/localtime
```

### 3. 构建系统快照

```bash
sudo starsleep build
```

也可以从外部配置目录复制后构建：

```bash
sudo starsleep build -cp /path/to/your/config
```

构建流程:
1. 创建展平 Btrfs 子卷
2. 按配置顺序逐层挂载 OverlayFS（lower=展平子卷, upper=层 diff）
3. 在合并视图中调用对应 helper 执行同步
4. 卸载后将 upper 层通过 reflink 展平合并到子卷
5. 所有层完成后生成 Btrfs 快照，更新 `latest` 符号链接

可选参数:
- `--clean` — 清理所有层缓存后重新构建
- `--clean layer1 layer2` — 仅清理指定层
- `--verify` — 构建后校验展平一致性

### 4. 部署引导

```bash
sudo starsleep flatten
```

将最新快照的内核和 initramfs 复制到 `/boot/starsleep/<快照名>/`，并在 `/boot/loader/entries/` 生成 systemd-boot 引导条目。

```bash
sudo starsleep flatten <快照名>     # 部署指定快照
sudo starsleep flatten --list        # 列出已部署的引导条目
sudo starsleep flatten --remove <名> # 移除引导条目
```

### 5. 动态维护（可选）

在已启动的 StarSleep 快照中，直接同步配置到当前运行系统：

```bash
sudo starsleep maintain
```

此命令会:
1. 清理不在配置中的多余软件包和孤立依赖
2. 安装官方仓库和 AUR 软件包
3. 启用/禁用 systemd 服务
4. 叠加配置文件
5. 执行 chroot 命令
6. 对当前根目录创建 Btrfs 快照并部署引导

## 命令参考

| 命令 | 说明 |
|---|---|
| `starsleep init` | 初始化工作目录和依赖检查 |
| `starsleep build` | 分层构建系统快照 |
| `starsleep flatten` | 部署快照到 systemd-boot |
| `starsleep maintain` | 动态维护当前运行系统 |
| `starsleep verify` | 独立运行展平一致性校验 |

通用选项:

| 选项 | 说明 |
|---|---|
| `-c, --config <路径>` | 指定配置目录（默认 `/starsleep/config`） |
| `-cp, --copy <路径>` | 复制外部配置到默认路径后运行 |
| `--lang <zh\|en>` | 指定界面语言（默认自动检测） |

## 工作原理

```
  ┌──────────────────────────────────────────────────┐
  │           Layer YAML 配置 (按文件名排序)           │
  └──────────────┬───────────────────────────────────┘
                 │
  ┌──────────────▼───────────────────────────────────┐
  │  逐层构建循环                                      │
  │  ┌────────────────────────────────────────────┐  │
  │  │ OverlayFS mount                            │  │
  │  │   lower = flat 子卷 (上轮展平结果)           │  │
  │  │   upper = layers/<name> (持久 diff)         │  │
  │  │   merged = work/merged                     │  │
  │  ├────────────────────────────────────────────┤  │
  │  │ 在 merged 中调用 helper 同步软件包/服务       │  │
  │  ├────────────────────────────────────────────┤  │
  │  │ umount → reflink 展平 upper → flat 子卷     │  │
  │  └────────────────────────────────────────────┘  │
  └──────────────┬───────────────────────────────────┘
                 │
  ┌──────────────▼───────────────────────────────────┐
  │  btrfs subvolume snapshot flat → snapshots/<ts>  │
  ├──────────────────────────────────────────────────┤
  │  应用 inherit.list → 继承宿主文件                  │
  ├──────────────────────────────────────────────────┤
  │  flatten → 复制内核到 ESP + 生成 boot entry       │
  └──────────────────────────────────────────────────┘
```

每层的 upper diff 目录被持久保留，后续构建仅需重新同步变更的层，未修改的层直接复用，从而实现增量构建。

## 层配置 YAML 格式

```yaml
name: <层名称>           # 必填，唯一标识
helper: <工具类型>        # 必填，pacstrap | pacman | paru | enable_service | copy_files | chroot-cmd | chroot-pacman
env:                     # 可选，chroot-cmd / chroot-pacman 使用
  - key: <环境变量名>
    value: <固定值>       # 与 host_key 二选一，value 优先
    host_key: <主机变量名>  # 从主机继承环境变量
packages:                # helper 为 pacstrap/pacman/paru/chroot-pacman 时使用
  - <包名>
services:                # helper 为 enable_service 时使用
  - <服务名>
files:                   # helper 为 copy_files 时使用
  - src: <相对于 config/files/ 的路径>
    dst: <目标系统路径>
commands:                # helper 为 chroot-cmd 时使用
  - <命令字符串>
```

### Helper 类型

| Helper | 用途 | 说明 |
|---|---|---|
| `pacstrap` | 初始化基础根文件系统 | 首次全新安装使用 `pacstrap -K -c`，已有系统时退化为 pacman 增量同步 |
| `pacman` | 安装官方仓库软件包 | 使用 `pacman -S --needed --noconfirm`，同时声明式清理多余包 |
| `paru` | 安装 AUR 软件包 | 通过 `runuser -u builder` 以非 root 用户调用 paru |
| `enable_service` | 启用 systemd 服务 | 使用 `systemctl --root enable`，自动禁用不在配置中的多余服务 |
| `copy_files` | 叠加配置文件 | 将 `config/files/` 下的文件/目录复制到目标系统指定位置，路径防穿越 |
| `chroot-cmd` | 在 chroot 中执行命令 | 通过 `arch-chroot` 运行任意命令，支持环境变量预设 |
| `chroot-pacman` | 在 chroot 中安装包 | 通过 `arch-chroot` 运行 pacman，适用于需 chroot 环境的包（如内核） |

## 许可证

BSD-3-Clause

