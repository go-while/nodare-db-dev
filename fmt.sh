go fmt *.go
cd client && go fmt *.go && echo " ^^client/ fmt ok" && cd .. || exit 1
cd database && go fmt *.go && echo " ^^database/ fmt ok" && cd .. || exit 1
cd logger && go fmt *.go && echo " ^^logger/ fmt ok" && cd .. || exit 1
cd server && go fmt *.go && echo " ^^server/ fmt ok" && cd .. || exit 1
cd utils && go fmt *.go && echo " ^^utils/ fmt ok" && cd .. || exit 0




