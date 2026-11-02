package finance

import (
	"errors"
	"math/rand"
	"testing"
	"time"
)

func sum64(m map[string]int64) (t int64) {
	for _, v := range m {
		t += v
	}
	return
}

// Propriedade: a soma das partes de qualquer divisão é exatamente o valor (o risco mais caro da fase).
func TestSharesAlwaysSumToTheAmount(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 20000; i++ {
		amount := rng.Int63n(MaxAmountCents) + 1
		a := rng.Intn(PointsTotal + 1)
		split := map[string]int{"u-a": a, "u-b": PointsTotal - a}
		if got := sum64(Shares(amount, split)); got != amount {
			t.Fatalf("amount=%d split=%v: partes somam %d", amount, split, got)
		}
	}
}

func TestSharesRoundingIsDeterministic(t *testing.T) {
	// 1 centavo dividido igual: o centavo vai para o menor id
	got := Shares(1, map[string]int{"b": 5000, "a": 5000})
	if got["a"] != 1 || got["b"] != 0 {
		t.Errorf("o centavo sobrante deveria ir para o menor id: %v", got)
	}
	// 100 em 33,33%/66,67%: 33,33 e 66,67 exatos
	got = Shares(100, map[string]int{"a": 3333, "b": 6667})
	if got["a"] != 33 || got["b"] != 67 {
		t.Errorf("33,33%% de 100 = 33 e 66,67%% = 67: %v", got)
	}
	// 10 em três partes iguais não existe com duas pessoas, mas 3 centavos em 50/50: o maior resto fica com o extra
	got = Shares(3, map[string]int{"a": 5000, "b": 5000})
	if got["a"]+got["b"] != 3 || got["a"] != 2 {
		t.Errorf("3 centavos 50/50: %v", got)
	}
	// maior parte fracionária perdida: 10 centavos, 30%/70% = 3/7 exatos; 25 centavos, 33,33/66,67
	got = Shares(25, map[string]int{"a": 3333, "b": 6667}) // 8,3325 e 16,6675
	if got["a"] != 8 || got["b"] != 17 {
		t.Errorf("25 centavos em 33,33/66,67 deveria dar 8 e 17: %v", got)
	}
}

func TestSplitInstallmentsSumsExactlyAndExtrasGoFirst(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 5000; i++ {
		count := rng.Intn(47) + 2
		total := rng.Int63n(MaxAmountCents) + int64(count)
		parts := SplitInstallments(total, count)
		var sum int64
		for j, p := range parts {
			sum += p
			if j > 0 && p > parts[j-1] {
				t.Fatalf("os centavos extras vão às primeiras parcelas: %v", parts)
			}
			if parts[0]-p > 1 {
				t.Fatalf("as parcelas diferem em no máximo 1 centavo: %v", parts)
			}
		}
		if sum != total {
			t.Fatalf("total=%d count=%d: soma %d", total, count, sum)
		}
	}
	if got := SplitInstallments(1000, 3); got[0] != 334 || got[1] != 333 || got[2] != 333 {
		t.Errorf("R$ 10,00 em 3 vezes: %v", got)
	}
}

func TestAddMonthsKeepsAnchorAndClampsShortMonths(t *testing.T) {
	d := func(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, time.UTC) }
	jan31 := d(2027, time.January, 31)
	cases := []struct {
		months int
		want   time.Time
	}{
		{1, d(2027, time.February, 28)}, {2, d(2027, time.March, 31)}, {3, d(2027, time.April, 30)},
		{12, d(2028, time.January, 31)}, {13, d(2028, time.February, 29)}, // 2028 é bissexto
	}
	for _, c := range cases {
		if got := AddMonths(jan31, c.months, 31); !got.Equal(c.want) {
			t.Errorf("31/jan + %d meses = %s, esperava %s", c.months, got.Format(dateLayout), c.want.Format(dateLayout))
		}
	}
	if got := DayInMonth(d(2027, time.February, 10), 31); got.Day() != 28 {
		t.Errorf("dia 31 em fevereiro cai no último dia: %v", got)
	}
}

func TestProportionalSplit(t *testing.T) {
	ms := []string{"a", "b"}
	// 1/3 e 2/3 dão dízima: precisam somar exatamente 10000
	sp, ok := ProportionalSplit(ms, map[string]int64{"a": 100000, "b": 200000})
	if !ok || sp["a"]+sp["b"] != PointsTotal || sp["a"] != 3333 || sp["b"] != 6667 {
		t.Errorf("1/3 e 2/3: %v %v", sp, ok)
	}
	rng := rand.New(rand.NewSource(3))
	for i := 0; i < 5000; i++ {
		sp, ok := ProportionalSplit(ms, map[string]int64{"a": rng.Int63n(MaxAmountCents) + 1, "b": rng.Int63n(MaxAmountCents) + 1})
		if !ok || sp["a"]+sp["b"] != PointsTotal {
			t.Fatalf("a divisão proporcional precisa somar %d: %v", PointsTotal, sp)
		}
	}
	for name, in := range map[string]map[string]int64{
		"falta uma renda": {"a": 100},
		"renda zero":      {"a": 100, "b": 0},
	} {
		if _, ok := ProportionalSplit(ms, in); ok {
			t.Errorf("%s: não deveria haver divisão", name)
		}
	}
	if _, ok := ProportionalSplit([]string{"a"}, map[string]int64{"a": 100}); ok {
		t.Error("com uma pessoa só não há divisão proporcional")
	}
}

func TestNormalizeSplit(t *testing.T) {
	ms := []string{"a", "b"}
	got, err := NormalizeSplit(map[string]int{"a": 10000}, ms)
	if err != nil || got["a"] != 10000 || got["b"] != 0 {
		t.Errorf("0/100 é válido e completa as pessoas: %v %v", got, err)
	}
	for name, in := range map[string]map[string]int{
		"não soma 10000": {"a": 5000, "b": 4000},
		"de fora":        {"a": 5000, "x": 5000},
		"negativa":       {"a": -1, "b": 10001},
		"vazia":          {},
	} {
		if _, err := NormalizeSplit(in, ms); !errors.Is(err, ErrSplitInvalid) {
			t.Errorf("%s: esperava split_invalid, veio %v", name, err)
		}
	}
}

func TestComputeBalanceIsSymmetricAndExact(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	for i := 0; i < 2000; i++ {
		var es []Expense
		for j := 0; j < rng.Intn(15)+1; j++ {
			a := rng.Intn(PointsTotal + 1)
			payer := []string{"a", "b"}[rng.Intn(2)]
			es = append(es, Expense{PaidBy: payer, AmountCents: rng.Int63n(5_000_000) + 1,
				Split: map[string]int{"a": a, "b": PointsTotal - a}})
		}
		from, to, owed, paid := ComputeBalance(es)
		// confere do outro lado: quem deve fica com o saldo líquido igual a -owed e o outro a +owed
		net := map[string]int64{}
		for _, e := range es {
			net[e.PaidBy] += e.AmountCents
			for id, s := range Shares(e.AmountCents, e.Split) {
				net[id] -= s
			}
		}
		if net["a"]+net["b"] != 0 {
			t.Fatalf("o saldo dos dois precisa se anular: %v", net)
		}
		if owed == 0 {
			if net["a"] != 0 {
				t.Fatalf("sem dívida, o saldo é zero: %v", net)
			}
			continue
		}
		if net[from] != -owed || net[to] != owed || from == to {
			t.Fatalf("from=%s to=%s owed=%d net=%v", from, to, owed, net)
		}
		if paid["a"]+paid["b"] <= 0 {
			t.Fatal("paid deveria somar o que foi gasto")
		}
	}
}

func TestSafeCellStopsSpreadsheetFormulas(t *testing.T) {
	for in, want := range map[string]string{"=1+1": "'=1+1", "+cmd": "'+cmd", "-x": "'-x", "@x": "'@x", "mercado": "mercado", "": ""} {
		if got := safeCell(in); got != want {
			t.Errorf("safeCell(%q) = %q, esperava %q", in, got, want)
		}
	}
}
