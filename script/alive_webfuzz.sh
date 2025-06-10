#!/bin/bash

# ========== 参数解析 ==========
PROXY_ARG=""
while getopts ":x:" opt; do
  case $opt in
    x)
      # 注意去除前面的等号（比如 -x=http://... 传入时可能带=）
      proxy_val="${OPTARG#*=}"
      if [[ -z "$proxy_val" ]]; then
        proxy_val="$OPTARG"
      fi
      PROXY_ARG="--proxy $proxy_val"
      echo "✅ 使用代理：$proxy_val"
      ;;
    *)
      echo "❌ 未知参数"
      exit 1
      ;;
  esac
done

# ========== 路径定义 ==========
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
YAML_FILE="$SCRIPT_DIR/../source/webfuzz.yaml"
RESULT_FILE="$SCRIPT_DIR/result.log"

if [ ! -f "$YAML_FILE" ]; then
  echo "❌ 节点列表文件不存在：$YAML_FILE"
  exit 1
fi

# ========== 时间阈值定义 ==========
month=2592000
timestamp=$(date +%s)
lastMonth=$(( timestamp - month ))

# 初始化结果文件
echo "# 自动检测生成 $(date +%Y%m%d)" > "$RESULT_FILE"

# 获取 YAML 项数
count=$(yq e 'length' "$YAML_FILE")

# ========== 开始检测 ==========
for ((i = 0; i < count; i++)); do
  item=$(yq e ".[$i]" "$YAML_FILE")
  url=$(echo "$item" | yq e '.options.url' -)

  echo "🔍 检测 URL: $url"

  if [[ $url == https://raw.githubusercontent.com/* ]]; then
    # 检查 URL 是否有效
    code=$(curl $PROXY_ARG -o /dev/null -s -w "%{http_code}" "$url")
    if [[ "$code" == "404" ]]; then
      echo "❌ 文件不存在：$url"
      continue
    fi

    # 路径解析（与 nodelist.txt 保持一致）
    repo_path=$(echo "$url" | cut -d '/' -f 4,5)
    repo_api="https://api.github.com/repos/$repo_path"

    # 获取更新时间
    response=$(curl $PROXY_ARG -s -H "User-Agent: alive-script" "$repo_api")
    updated_at=$(echo "$response" | jq -r '.updated_at // empty')

    if [[ -z "$updated_at" ]]; then
      echo "⚠️ 无法获取更新时间：$repo_api"
      continue
    fi

    # 是否在30天内
    if date --version >/dev/null 2>&1; then
      updated_ts=$(date -d "$updated_at" +%s)
    else
      updated_ts=$(date -j -f "%Y-%m-%dT%H:%M:%SZ" "$updated_at" +%s)
    fi
    if [[ $updated_ts -gt $lastMonth ]]; then
      echo "- type: webfuzz" >> "$RESULT_FILE"
      echo "  options:" >> "$RESULT_FILE"
      echo "    url: $url" >> "$RESULT_FILE"
      echo "" >> "$RESULT_FILE"
      echo "✅ 保留：$url（更新时间：$updated_at）"
    else
      echo "⏸️ 仓库太久未更新：$url（$updated_at）"
    fi
  else
    echo "⛔ 暂不支持的 URL：$url"
  fi
done

echo "✅ 完成。活跃条目写入：$RESULT_FILE"