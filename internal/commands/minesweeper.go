package commands

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

var (
	numbers = map[int]string{
		-1: "💣",
		0:  "0️⃣",
		1:  "1️⃣",
		2:  "2️⃣",
		3:  "3️⃣",
		4:  "4️⃣",
		5:  "5️⃣",
		6:  "6️⃣",
		7:  "7️⃣",
		8:  "8️⃣",
	}
)

type Minesweeper struct {
	Field [10][10]int
	Bombs []*Point
}

type Point struct {
	x int
	y int
}

func (m *Minesweeper) generateField() {
	rand.Seed(time.Now().UnixNano())

	count := 0
	for count < 20 {
		x := rand.Intn(10)
		y := rand.Intn(10)
		for m.isBomb(&Point{x, y}) {
			x = rand.Intn(10)
			y = rand.Intn(10)
		}
		count++
		m.plantBomb(&Point{x, y})
	}

	for _, bomb := range m.Bombs {
		n := m.neighbors(bomb)
		for _, p := range n {
			m.Field[p.x][p.y]++
		}
	}
}

func (m *Minesweeper) isBomb(p *Point) bool {
	return m.Field[p.x][p.y] == -1
}

func (m *Minesweeper) plantBomb(p *Point) {
	m.Bombs = append(m.Bombs, p)
	m.Field[p.x][p.y] = -1
}

func (m *Minesweeper) neighbors(p *Point) []*Point {
	all := []*Point{
		{p.x - 1, p.y}, {p.x + 1, p.y}, {p.x - 1, p.y + 1}, {p.x - 1, p.y - 1}, {p.x, p.y - 1}, {p.x, p.y + 1}, {p.x + 1, p.y + 1}, {p.x + 1, p.y - 1},
	}

	valid := make([]*Point, 0)
	for _, n := range all {
		if m.isValid(n) {
			valid = append(valid, n)
		}
	}
	return valid
}

func (m *Minesweeper) isValid(p *Point) bool {
	if p.x >= 10 || p.x < 0 {
		return false
	}

	if p.y >= 10 || p.y < 0 {
		return false
	}

	if m.isBomb(p) {
		return false
	}

	return true
}

func (m *Minesweeper) String() string {
	var sb strings.Builder

	for _, row := range m.Field {
		for _, col := range row {
			sb.WriteString(fmt.Sprintf("||%v||", numbers[col]))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
