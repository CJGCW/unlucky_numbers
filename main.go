package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
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

func PrettyPrintBoardsGridCentered(state *GameState) {
	cellWidth := 5
	repeat := func(s string, n int) string {
		res := ""
		for i := 0; i < n; i++ {
			res += s
		}
		return res
	}

	// --- Print Table header ---
	boardWidth := BoardSize*(cellWidth+1) + 1
	totalWidth := boardWidth*len(state.Boards) + (len(state.Boards)-1)*2 // spaces between boards

	tableHeader := " TABLE "
	dashesEachSide := (totalWidth - len(tableHeader)) / 2
	fmt.Println("+" + repeat("-", dashesEachSide) + tableHeader + repeat("-", totalWidth-len(tableHeader)-dashesEachSide) + "+")

	// --- Print Table contents ---
	tableTiles := append([]int{}, state.Table...)
	sort.Ints(tableTiles)
	content := ""
	for i, t := range tableTiles {
		if i > 0 {
			content += ","
		}
		content += fmt.Sprintf("%d", t)
	}
	if content == "" {
		content = "(empty)"
	}
	padding := (totalWidth - len(content)) / 2
	fmt.Println("|" + repeat(" ", padding) + content + repeat(" ", totalWidth-len(content)-padding) + "|")
	fmt.Println("+" + repeat("-", totalWidth) + "+")

	// --- Print Boards ---
	for i := range state.Boards {
		header := fmt.Sprintf("Player %d", i)
		padding := (boardWidth - len(header)) / 2
		fmt.Printf("%s%s%s", repeat(" ", padding), header, repeat(" ", boardWidth-len(header)-padding))
		if i < len(state.Boards)-1 {
			fmt.Print("  ")
		}
	}
	fmt.Println()

	hLine := func(width int) string {
		line := "+"
		for i := 0; i < width; i++ {
			line += repeat("-", cellWidth) + "+"
		}
		return line
	}

	for r := 0; r < BoardSize; r++ {
		for i := range state.Boards {
			fmt.Print(hLine(BoardSize))
			if i < len(state.Boards)-1 {
				fmt.Print("  ")
			}
		}
		fmt.Println()

		for i, b := range state.Boards {
			fmt.Print("|")
			for c := 0; c < BoardSize; c++ {
				v := b.Grid[r][c]
				content := "."
				if v != 0 {
					content = fmt.Sprintf("%d", v)
				}
				spaces := cellWidth - len(content)
				left := spaces / 2
				right := spaces - left
				fmt.Print(repeat(" ", left) + content + repeat(" ", right) + "|")
			}
			if i < len(state.Boards)-1 {
				fmt.Print("  ")
			}
		}
		fmt.Println()
	}

	for i := range state.Boards {
		fmt.Print(hLine(BoardSize))
		if i < len(state.Boards)-1 {
			fmt.Print("  ")
		}
	}
	fmt.Println()
}

func contains(slice []int, val int) bool {
    for _, v := range slice {
        if v == val {
            return true
        }
    }
    return false
}

func getNumberOfPlayers() int {
	var numPlayers int
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter number of players (2–4, default 2): ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			numPlayers = 2
			break
		}
		n, err := strconv.Atoi(line)
		if err == nil && n >= 2 && n <= 4 {
			numPlayers = n
			break
		}
		fmt.Println("Invalid input.")
	}
	return numPlayers
}

func (state *GameState)setUpBoards(numPlayers int)  {
	reader := bufio.NewReader(os.Stdin)
	// --- Board setup ---
	fmt.Print("Do you want to set up the boards manually? (y/N): ")
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	setupManual := line == "y" || line == "yes"

	var setupType string
	if setupManual {
		for {
			fmt.Print("Initial (diagonal numbers) or advanced (enter r,c,t arrays)? (i/a, default i): ")
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(strings.ToLower(line))
			if line == "" {
				setupType = "i"
				break
			}
			if line == "i" || line == "a" {
				setupType = line
				break
			}
		}
	}

	for p := 0; p < numPlayers; p++ {
		b := &Board{}
		if setupManual {
			if setupType == "a" {
				fmt.Printf("Enter tiles for Player %d as r,c,t; separate multiple with ;\n", p)
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(input)
				if input != "" {
					moves := strings.Split(input, ";")
					for _, m := range moves {
						parts := strings.Split(strings.TrimSpace(m), ",")
						if len(parts) == 3 {
							r, err1 := strconv.Atoi(parts[0])
							c, err2 := strconv.Atoi(parts[1])
							t, err3 := strconv.Atoi(parts[2])
							if err1 == nil && err2 == nil && err3 == nil &&
								r >= 0 && r < BoardSize && c >= 0 && c < BoardSize {
								b.Grid[r][c] = t
							}
						}
					}
				}
			} else { // initial
				fmt.Printf("Enter 4 numbers for Player %d diagonal positions: ", p)
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(input)
				input = strings.ReplaceAll(input, ",", " ")
				nums := strings.Fields(input)
				for i := 0; i < BoardSize && i < len(nums); i++ {
					t, err := strconv.Atoi(nums[i])
					if err == nil {
						b.Grid[i][i] = t
					} else {
						fmt.Println(err)
					}
					
				}
			}
		} else {
			available := rand.Perm(20)
			for i := 0; i < BoardSize; i++ {
				b.Grid[i][i] = available[i] + 1
			}
		}
		state.Boards = append(state.Boards, b)
	}
}

func (state *GameState) playGame() {
		var current int
		reader := bufio.NewReader(os.Stdin)
		numPlayers := len(state.Boards)
	for {
		fmt.Printf("Who starts (0–%d, default 0): ", numPlayers-1)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			current = 0
			break
		}
		n, err := strconv.Atoi(line)
		if err == nil && n >= 0 && n < numPlayers {
			current = n
			break
		}
	}
	for {
    	fmt.Printf("\nPlayer %d's turn — enter drawn tile or 'p' to pick from table (blank to quit): ", current)
    	line, _ := reader.ReadString('\n')
    	line = strings.TrimSpace(line)
    	if line == "" {
    	    fmt.Println("Exiting game.")
    	    return
    	}

    	var tile int
    	if line == "p" {
    	    if len(state.Table) == 0 {
    	        fmt.Println("Table is empty, cannot pick.")
    	        continue
    	    }
    	    // show table options
    	    fmt.Println("Tiles on table:", state.Table)
    	    fmt.Print("Enter tile to pick: ")
    	    choice, _ := reader.ReadString('\n')
    	    choice = strings.TrimSpace(choice)
    	    t, err := strconv.Atoi(choice)
    	    if err != nil || !contains(state.Table, t) {
    	        fmt.Println("Invalid choice.")
    	        continue
    	    }
    	    tile = t
    	    // remove from table
    	    for i, v := range state.Table {
    	        if v == tile {
    	            state.Table = append(state.Table[:i], state.Table[i+1:]...)
    	            break
    	        }
    	    }
    	} else {
    	    t, err := strconv.Atoi(line)
    	    if err != nil {
    	        fmt.Println("Invalid tile.")
    	        continue
    	    }
    	    tile = t
    	}

		state.playerTurn(current, tile)
		

		PrettyPrintBoardsGridCentered(state)
		current = (current + 1) % len(state.Boards)
	}
}

func (state *GameState) playerTurn(current int, tile int) {
		reader := bufio.NewReader(os.Stdin)
    	board := state.Boards[current]
	for {
			fmt.Printf("Action for %d? ([r]ecommend, [t]able, or row,col): ", tile)
			action, _ := reader.ReadString('\n')
			action = strings.TrimSpace(action)

			if action == "t" {
				state.Table = append(state.Table, tile)
				fmt.Println("Placed on table.")
				break
			}

			if action == "r" {
				recs := bestMoves(board, tile, state, current)
				if len(recs) == 0 {
					fmt.Println("No legal placements found.")
					continue // stay in loop
				}
				for i, m := range recs {
					fmt.Printf("%d) %s at (%d,%d) — score %.2f\n",
						i+1,
						map[MoveType]string{Place: "Place", Swap: "Swap"}[m.Type],
						m.Cell.R, m.Cell.C, m.Score)
				}
				fmt.Print("Choose a move number or press Enter to skip: ")
				choice, _ := reader.ReadString('\n')
				choice = strings.TrimSpace(choice)
				if choice == "" {
					continue
				}
				idx, err := strconv.Atoi(choice)
				if err == nil && idx >= 1 && idx <= len(recs) {
					ApplyMove(board, recs[idx-1], tile)
					fmt.Println("Move applied.")
					break
				} else {
					fmt.Println("Invalid choice.")
					continue
				}
			}

			parts := strings.Split(action, ",")
			if len(parts) == 2 {
				r, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
				c, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
				if err1 == nil && err2 == nil && r >= 0 && r < BoardSize && c >= 0 && c < BoardSize {
				    if isLegalPlacement(board, tile, r, c) {
				        old := board.Grid[r][c]
				        board.Grid[r][c] = tile
				        if old != 0 {
				            state.Table = append(state.Table, old)
				            fmt.Printf("Swapped %d into table, placed %d at (%d,%d).\n", old, tile, r, c)
				        } else {
				            fmt.Printf("Placed %d at (%d,%d).\n", tile, r, c)
				        }
				        break
				    } else {
				        fmt.Println("Illegal placement, try again.")
				    }
				}

			}

			fmt.Println("Invalid input, try again.")
		}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	var numPlayers = getNumberOfPlayers()
	
	
	state := &GameState{Boards: []*Board{}}

	state.setUpBoards(numPlayers)

	PrettyPrintBoardsGridCentered(state)

	// --- Starting player ---
	
	state.playGame()
	

	// --- Main game loop ---
	
}
