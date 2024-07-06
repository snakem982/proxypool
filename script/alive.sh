#!/bin/bash

while getopts ":x:" opt
do
    case $opt in
        x)
        export http_proxy=$OPTARG
        export https_proxy=$http_proxy
        echo "使用代理 $https_proxy"
        ;;
        ?)
        echo "未知参数"
        exit 1;;
    esac
done

jsonQ() {
    json=$(cat)
    awk -v json="$json" -v json_original="$json" -v key="$1" '
    function strLastChar(s) {
        return substr(s, length(s), 1)
    }
    function startWith(s, c) {
        start = substr(s, 1, 1)
        return start == c
    }
    function endWith(s, c) {
        return strLastChar(s) == c
    }
    function innerStr(s) {
        # 取出括号/引号内的内容
        return substr(s, 2, length(s)-2)
    }
    function strIndex(s, n) {
        # 字符串通过下标取值，索引是从1开始的
        return substr(s, n, 1)
    }
    function trim(s) {
        sub("^[ \n]*", "", s);
        sub("[ \n]*$", "", s);
        return s
    }
    function findValueByKey(s, k) {
        if ("\""k"\"" != substr(s, 1, length(k)+2)) {exit 0}
        s = trim(s)
        start = 0; stop = 0; layer = 0
        for (i = 2 + length(k) + 1; i <= length(s); ++i) {
            lastChar = substr(s, i - 1, 1)
            currChar = substr(s, i, 1)
            if (start <= 0) {
                if (lastChar == ":") {
                    start = currChar == " " ? i + 1: i
                    if (currChar == "{" || currChar == "[") {
                        layer = 1
                    }
                }
            } else {
                if (currChar == "{" || currChar == "[") {
                    ++layer
                }
                if (currChar == "}" || currChar == "]") {
                    --layer
                }
                if ((currChar == "," || currChar == "}" || currChar == "]") && layer <= 0) {
                    stop = currChar == "," ? i : i + 1 + layer
                    break
                }
            }
        }
        if (start <= 0 || stop <= 0 || start > length(s) || stop > length(s) || start >= stop) {
            exit 0
        } else {
            return trim(substr(s, start, stop - start))
        }
    }
    function unquote(s) {
        if (startWith(s, "\"")) {
            s = substr(s, 2, length(s)-1)
        }
        if (endWith(s, "\"")) {
            s = substr(s, 1, length(s)-1)
        }
        return s
    }
    BEGIN{
        if (match(key, /^\./) == 0) {exit 0;}
        sub(/\][ ]*\[/,"].[", key)
        split(key, ks, ".")
        if (length(ks) == 1) {print json; exit 0}
        for (j = 2; j <= length(ks); j++) {
            k = ks[j]
            if (startWith(k, "[") && endWith(k, "]") == 1) { # [n]
                idx = innerStr(k)
                currentIdx = -1
                # 找匹配对
                pairs = ""
                json = trim(json)
                if (startWith(json, "[") == 0) {
                    exit 0
                }
                start = 2
                cursor = 2
                for (; cursor <= length(json); cursor++) {
                    current = strIndex(json, cursor)
                    if (current == " " || current == "\n") {continue}
                    # 忽略空白
                    if (current == "[" || current == "{") {
                        if (length(pairs) == 0) {start = cursor}
                        pairs = pairs""current
                    }
                    if (current == "]" || current == "}") {
                        if ((strLastChar(pairs) == "[" && current == "]") || (strLastChar(pairs) == "{" && current == "}")) {
                            pairs = substr(pairs, 1, length(pairs)-1)
                            # 删掉最后一个字符
                            if (pairs == "") {
                                # 匹配到了所有的左括号
                                currentIdx++
                                if (currentIdx == idx) {
                                    json = substr(json, start, cursor-start+1)
                                    break
                                }
                            }
                        } else {
                            pairs = pairs""current
                        }
                    }
                }
            } else {
                # 到这里，就只能是{"key": "value"}或{"key":{}}或{"key":[{}]}
                pairs = ""
                json = trim(json)
                if (startWith(json, "[")) {exit 0}
                #if (!startWith(json, "\"") || !startWith(json, "{")) {json="\""json}
                # 找匹配的键
                start = 2
                cursor = 2
                noMatch = 0
                for (; cursor <= length(json); cursor++) {
                    current = strIndex(json, cursor)
                    if (current == " " || current == "\n" || current == ",") {continue}
                    # 忽略空白和逗号
                    if (substr(json, cursor, length(k)+2) == "\""k"\"") {
                        json = findValueByKey(substr(json, cursor, length(json)-cursor+1), k)
                        break
                    } else {
                        noMatch = 1
                    }
                    if (noMatch) {
                        pos = match(substr(json, cursor+1, length(json)-cursor), /[^(\\")]"/)
                        ck = substr(substr(json, cursor+1, length(json)-cursor), 1, pos)
                        t = findValueByKey(substr(json, cursor, length(json)-cursor+1), ck)
                        tLen = length(t)
                        sub(/\\/, "\\\\", t)
                        pos = match(substr(json, cursor+1, length(json)-cursor), t)
                        if (pos != 0) {
                            cursor = cursor + pos + tLen
                        }
                        noMatch = 0
                        continue
                    }
                }
            }
        }
        if (json_original == json) { print;exit 0 }
        print unquote(json)
    }'
}

month=2592000
timestamp=$(date +%s)
lastMonth=$(( timestamp-month ))
function isUpdated()
{
    t1=`gdate -d "$1" +%s`
    if [ $t1 -gt $lastMonth ]
    then
        return 0
    else
        return 1
    fi
}

rm -rf result.log
i=1

data=$(cat nodelist.txt)
for url in $data; do
  if [[ $url == https://raw.githubusercontent.com* ]]; then
    i=$[$i+1]
    code=$(curl -o /dev/null -k -s -w %{http_code} $url)
    if [ $code -ne 404 ]
    then
          arr=(`echo $url | tr '/' ' '`)
          repository=${arr[2]}%2F${arr[3]}
          response=$(curl -s https://github.com/search?q=$repository&type=repositories)
          response=$(echo "$response" | jsonQ ".payload.results.[0].repo.repository.updated_at")
          response=${response: 0: 19}
          response=${response/T/' '}
          if  isUpdated $response ; then
              echo $url >> result.log
          fi
    fi

    ix=`expr $i % 8`
    if [ $ix -eq 0 ]
    then
      echo stop
      sleep 120
    fi
  fi
done



