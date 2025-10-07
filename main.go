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
	Type    MoveType
	Cell    *Cell
	OldTile int     // only set if Type==Swap
	Score   float64 // for ranking
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

func isPlacementFeasible(board *Board, r, c, tile int, remaining map[int]int) bool {
	// Check above
	for rr := r - 1; rr >= 0; rr-- {
		v := board.Grid[rr][c]
		if v != 0 && tile <= v {
			return false
		}
	}

	// Check left
	for cc := c - 1; cc >= 0; cc-- {
		v := board.Grid[r][cc]
		if v != 0 && tile <= v {
			return false
		}
	}

	// Check downward feasibility
	minNeeded := tile + 1
	for rr := r + 1; rr < BoardSize; rr++ {
		v := board.Grid[rr][c]
		if v != 0 {
			if v <= tile {
				return false
			}
			break
		}
		// don't instantly fail just because remaining[minNeeded] == 0,
		// instead, look ahead for *any* available larger tile
		hasFuture := false
		for t := minNeeded; t <= 20; t++ {
			if remaining[t] > 0 {
				hasFuture = true
				break
			}
		}
		if !hasFuture {
			return false
		}
		minNeeded++
	}

	// Check rightward feasibility
	minNeeded = tile + 1
	for cc := c + 1; cc < BoardSize; cc++ {
		v := board.Grid[r][cc]
		if v != 0 {
			if v <= tile {
				return false
			}
			break
		}
		hasFuture := false
		for t := minNeeded; t <= 20; t++ {
			if remaining[t] > 0 {
				hasFuture = true
				break
			}
		}
		if !hasFuture {
			return false
		}
		minNeeded++
	}

	return true
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


// bestMoves returns the top N moves (placements or swaps) for a drawn tile
func bestMoves(board *Board, tile int, state *GameState, N int) []Move {
	type scoredMove struct {
		move Move
		score float64
	}
	moves := []scoredMove{}
	remaining := getRemainingTileCounts(state, board)

	// --- Placements ---
	for r := 0; r < BoardSize; r++ {
		for c := 0; c < BoardSize; c++ {
			if board.Grid[r][c] != 0 {
				continue
			}
			if !isLegalPlacement(board, tile, r, c) {
				continue
			}
			if !isPlacementFeasible(board, r, c, tile, remaining) {
				continue
			}
			// count options for scoring
			count := 0
			for t := 1; t <= 20; t++ {
				if remaining[t] > 0 && isLegalPlacement(board, t, r, c) && isPlacementFeasible(board, r, c, t, remaining) {
					count++
				}
			}
			if count == 0 {
				continue
			}
			score := 1.0 / float64(count)
			moves = append(moves, scoredMove{move: Move{Type: Place, Cell: &Cell{R: r, C: c}, Score: score}, score: score})
		}
	}

	// --- Swaps ---
	for r := 0; r < BoardSize; r++ {
		for c := 0; c < BoardSize; c++ {
			current := board.Grid[r][c]
			if current == 0 || current == tile {
				continue
			}
			// Swap only if new tile would fit here legally and be feasible
			tmp := *board
			tmp.Grid[r][c] = tile
			if !isLegalPlacement(&tmp, tile, r, c) {
				continue
			}
			if !isPlacementFeasible(&tmp, r, c, tile, remaining) {
				continue
			}
			// Also ensure the removed tile can still go elsewhere?
			// Optional: could check, but for now just scoring the swap placement
			count := 0
			for t := 1; t <= 20; t++ {
				if remaining[t] > 0 && isLegalPlacement(&tmp, t, r, c) && isPlacementFeasible(&tmp, r, c, t, remaining) {
					count++
				}
			}
			if count == 0 {
				continue
			}
			score := 1.0 / float64(count)
			moves = append(moves, scoredMove{move: Move{Type: Swap, Cell: &Cell{R: r, C: c}, OldTile: current, Score: score}, score: score})
		}
	}

	// --- Sort moves by score descending ---
	sort.Slice(moves, func(i, j int) bool {
		return moves[i].score > moves[j].score
	})

	// --- Return top N ---
	result := []Move{}
	for i := 0; i < len(moves) && i < N; i++ {
		result = append(result, moves[i].move)
	}
	return result
}


// getRemainingTileCounts excludes opponent tiles from availability
func getRemainingTileCounts(state *GameState, myBoard *Board) map[int]int {
	counts := make(map[int]int)
	numPlayers := len(state.Boards)
	for i := 1; i <= 20; i++ {
		counts[i] = 2 * numPlayers // two sets per player
	}

	for _, b := range state.Boards {
		for r := 0; r < BoardSize; r++ {
			for c := 0; c < BoardSize; c++ {
				if b.Grid[r][c] != 0 {
					if b == myBoard {
						// decrement normally
						counts[b.Grid[r][c]]--
					} else {
						// opponent tiles are "locked" — remove from remaining
						counts[b.Grid[r][c]] = 0
					}
				}
			}
		}
	}

	for _, t := range state.Table {
		counts[t]--
	}
	for _, d := range state.Draw {
		counts[d]--
	}

	// Make sure no negative counts
	for k, v := range counts {
		if v < 0 {
			counts[k] = 0
		}
	}
	return counts
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
		{0,0,8,19},
		{0,0,19,20},
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
	
	
	topMoves := bestMoves(board2, drawTile, state, 3)

	fmt.Printf("Top %d moves for tile %d:\n", len(topMoves), drawTile)
	for _, m := range topMoves {
		if m.Type == Place {
			fmt.Printf("Place at (%d,%d) — score %.2f\n", m.Cell.R, m.Cell.C, m.Score)
		} else if m.Type == Swap {
			fmt.Printf("Swap with %d at (%d,%d) — score %.2f\n", m.OldTile, m.Cell.R, m.Cell.C, m.Score)
		}
	}

	PrettyPrintBoardsGridCentered(state.Boards)
}
