package main

import (
	"fmt"
	"sort"
)

const BoardSize = 4

type Cell struct{ R, C int }

type MoveType int

const (
	Place MoveType = iota
	Swap
	Discard
)

type Move struct {
	Type MoveType
	Cell *Cell
}

type Board struct {
	Grid [BoardSize][BoardSize]int
}

type GameState struct {
	Boards []*Board
	Table  []int
	Draw   []int
}

// ---------------- Utility ----------------

func max(a, b int) int { if a > b { return a } else { return b } }
func min(a, b int) int { if a < b { return a } else { return b } }

// remainingTiles returns a set of tiles that are not on any board or on the table
func remainingTiles(state *GameState) map[int]bool {
	used := map[int]bool{}
	for _, b := range state.Boards {
		for r := 0; r < BoardSize; r++ {
			for c := 0; c < BoardSize; c++ {
				if b.Grid[r][c] != 0 {
					used[b.Grid[r][c]] = true
				}
			}
		}
	}
	for _, t := range state.Table {
		used[t] = true
	}
	remaining := map[int]bool{}
	for i := 1; i <= BoardSize*5; i++ { // 1-20 normally
		if !used[i] {
			remaining[i] = true
		}
	}
	return remaining
}
func isLegalPlacement(board *Board, val, r, c int) bool {

	// Check column above
	for rr := r - 1; rr >= 0; rr-- {
		if board.Grid[rr][c] != 0 {
			if val <= board.Grid[rr][c] {
				return false
			}
			break
		}
	}

	// Check column below
	for rr := r + 1; rr < BoardSize; rr++ {
		if board.Grid[rr][c] != 0 {
			if val >= board.Grid[rr][c] {
				return false
			}
			break
		}
	}

	// Check row left
	for cc := c - 1; cc >= 0; cc-- {
		if board.Grid[r][cc] != 0 {
			if val <= board.Grid[r][cc] {
				return false
			}
			break
		}
	}

	// Check row right
	for cc := c + 1; cc < BoardSize; cc++ {
		if board.Grid[r][cc] != 0 {
			if val >= board.Grid[r][cc] {
				return false
			}
			break
		}
	}

	return true
}


// legalRange returns min/max value that can go at (r,c)
func legalRange(board *Board, r, c int) (lo, hi int) {
	lo, hi = 1, 20
	if r > 0 && board.Grid[r-1][c] != 0 { lo = max(lo, board.Grid[r-1][c]+1) }
	if r < BoardSize-1 && board.Grid[r+1][c] != 0 { hi = min(hi, board.Grid[r+1][c]-1) }
	if c > 0 && board.Grid[r][c-1] != 0 { lo = max(lo, board.Grid[r][c-1]+1) }
	if c < BoardSize-1 && board.Grid[r][c+1] != 0 { hi = min(hi, board.Grid[r][c+1]-1) }
	return
}

// ---------------- AI Evaluation ----------------

// bestPlacement finds the best empty cell for tile, considering remaining tiles
func bestPlacement(board *Board, tile int, state *GameState) *Cell {
	var best *Cell
	bestScore := -1.0
	remaining := remainingTiles(state)

	for r := 0; r < BoardSize; r++ {
		for c := 0; c < BoardSize; c++ {
			if board.Grid[r][c] != 0 {
				continue
			}
			lo, hi := legalRange(board, r, c)
			if tile < lo || tile > hi {
				continue
			}
			// Count how many remaining tiles can fit in this cell
			count := 0
			for t := lo; t <= hi; t++ {
				if remaining[t] {
					count++
				}
			}
			if count == 0 { continue } // cannot fill in future
			score := 1.0 / float64(count) // fewer options = higher priority
			if score > bestScore {
				bestScore = score
				best = &Cell{r, c}
			}
		}
	}
	return best
}

func bestNPlacements(board *Board, tile int, state *GameState, N int) []*Cell {
	type scoredCell struct {
		cell  *Cell
		score float64
	}
	var candidates []scoredCell
	remaining := remainingTiles(state)

	for r := 0; r < BoardSize; r++ {
		for c := 0; c < BoardSize; c++ {
			if board.Grid[r][c] != 0 {
				continue
			}

			// Only consider truly legal placements
			if !isLegalPlacement(board, tile, r, c) {
				continue
			}

			// Count how many remaining tiles can fit in this cell
			count := 0
			for t := 1; t <= 20; t++ {
				if remaining[t] {
					if isLegalPlacement(board, t, r, c) {
						count++
					}
				}
			}
			if count == 0 {
				continue // placing here would block completion
			}

			score := 1.0 / float64(count) // fewer options = higher priority
			candidates = append(candidates, scoredCell{cell: &Cell{R: r, C: c}, score: score})
		}
	}

	// Sort descending by score
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	// Return top N
	top := []*Cell{}
	for i := 0; i < len(candidates) && i < N; i++ {
		top = append(top, candidates[i].cell)
	}
	return top
}


// FindBestMove returns the AI's best move
func FindBestMove(board *Board, tile int, state *GameState) Move {
	cell := bestPlacement(board, tile, state)
	if cell != nil {
		return Move{Type: Place, Cell: cell}
	}
	return Move{Type: Discard}
}

// ---------------- Game Utilities ----------------

func ApplyMove(board *Board, move Move, tile int) {
	if move.Type == Place || move.Type == Swap {
		board.Grid[move.Cell.R][move.Cell.C] = tile
	}
	if move.Type == Discard {
		// in full game, add to table
	}
}

// ---------------- Pretty Printing ----------------

func PrettyPrintBoardsGridCentered(boards []*Board) {
	cellWidth := 5
	repeat := func(s string, n int) string { res := ""; for i := 0; i < n; i++ { res += s }; return res }

	hLine := func() string { line := "+"; for i := 0; i < BoardSize; i++ { line += repeat("-", cellWidth) + "+" }; return line }

	// Header
	for i, _ := range boards {
		header := fmt.Sprintf("Player %d", i)
		totalWidth := BoardSize*(cellWidth+1) + 1
		padding := (totalWidth - len(header)) / 2
		fmt.Printf("%s%s%s", repeat(" ", padding), header, repeat(" ", totalWidth-len(header)-padding))
		if i < len(boards)-1 { fmt.Print("  ") }
	}
	fmt.Println()

	// Rows
	for r := 0; r < BoardSize; r++ {
		for i := range boards {
			fmt.Print(hLine())
			if i < len(boards)-1 { fmt.Print("  ") }
		}
		fmt.Println()

		for i, b := range boards {
			fmt.Print("|")
			for c := 0; c < BoardSize; c++ {
				v := b.Grid[r][c]
				content := "."
				if v != 0 { content = fmt.Sprintf("%d", v) }
				spaces := cellWidth - len(content)
				left := spaces / 2
				right := spaces - left
				fmt.Print(repeat(" ", left) + content + repeat(" ", right) + "|")
			}
			if i < len(boards)-1 { fmt.Print("  ") }
		}
		fmt.Println()
	}

	for i := range boards {
		fmt.Print(hLine())
		if i < len(boards)-1 { fmt.Print("  ") }
	}
	fmt.Println()
}

// ---------------- Example / Test ----------------

func main() {
	board1 := &Board{}
	board2 := &Board{}

	board1.Grid = [BoardSize][BoardSize]int{
		{5,0,0,9},
		{0,7,0,0},
		{0,0,8,0},
		{0,0,0,12},
	}
	board2.Grid = [BoardSize][BoardSize]int{
		{6,0,0,0},
		{0,10,0,0},
		{0,0,14,0},
		{0,0,0,20},
	}

	state := &GameState{
		Boards: []*Board{board1, board2},
		Table:  []int{7,5,17,4,10},
	}

	drawTile := 18
	
	move := FindBestMove(board2, drawTile, state)
if move.Cell != nil {
    fmt.Printf("Best move for tile %d: (%d,%d)\n", drawTile, move.Cell.R, move.Cell.C)
} else {
    fmt.Printf("Best move for tile %d: Discard\n", drawTile)
}
topCells := bestNPlacements(board2, drawTile, state, 3)
fmt.Printf("Top %d placements for tile %d:\n", len(topCells), drawTile)
for _, c := range topCells {
	fmt.Printf("  (%d,%d)\n", c.R, c.C)
}
	//ApplyMove(board2, move, drawTile)

	PrettyPrintBoardsGridCentered(state.Boards)
}
