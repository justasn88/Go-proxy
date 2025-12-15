Proxy serveris

Norint paleisti serveri reikia nueiti į failų kelia kuriame išsisaugojot projekta ir paleisti su komanda:

go run main.go


Pavyzdinės komandos su kuriomis galite naudoti proxy serverį:

curl -x http://user:pass@localhost:8080 http://example.com

curl -x http://user:pass@localhost:8080 https://google.com

Padarytas ir testas. Jį galima paleisti su komanda:
go test ./...
