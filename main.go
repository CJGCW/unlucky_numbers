package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"math"
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
	IsAi bool // the enemy!
}

type GameState struct {
	Boards       []*Board
	Table        []int
	Draw         []int
	Analyze      bool // analysis mode aka we tell it what numbers we draw.
	BrunoVariant bool
	Current      int //track player turns for save/load actions
}

var reader = bufio.NewReader(os.Stdin)

// ---------------- Utility ----------------

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

// ---------------- AI Evaluation ----------------
func placementScore(tile int, r, c int) float64 {
	// Normalize tile to 0..1 range (1..20)
	norm := float64(tile-1) / float64(BoardSize*BoardSize-1) // 0..1

	// Desired row/col for this tile
	desired := norm * float64(BoardSize-1) // 0..3

	// Penalize distance from desired row and col
	rowDiff := float64(r) - desired
	colDiff := float64(c) - desired

	score := -(rowDiff*rowDiff + colDiff*colDiff) // negative distance squared
	return score
}
func (state *GameState) isPlacementFeasible(board *Board, r, c, tile int) bool {
	remaining := append(state.Draw, state.Table...)
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
		for t := minNeeded; t < len(remaining); t++ {
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
		for t := 0; t < len(remaining); t++ {
			if remaining[t] >= minNeeded {
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

func (state *GameState) bestMoves(board *Board, tile int) []Move {
	var moves []Move

	// Precompute remaining tiles for feasibility checks
	remaining := make(map[int]bool)
	for _, t := range state.Draw {
		remaining[t] = true
	}
	for _, t := range state.Table {
		remaining[t] = true
	}

	for r := 0; r < BoardSize; r++ {
		for c := 0; c < BoardSize; c++ {
			if board.Grid[r][c] == 0 {
				move := Move{Type: Place, Cell: &Cell{R: r, C: c}}
				if state.isPlacementFeasible(board, r, c, tile) {
					move.Score = placementScore(r, c, tile) // your scoring function
					moves = append(moves, move)
				}
			} else {
				// Consider swap if swapping is legal
				old := board.Grid[r][c]
				move := Move{Type: Swap, Cell: &Cell{R: r, C: c}}
				if tile > old && state.isPlacementFeasible(board, r, c, tile) {
					move.Score = placementScore(r, c, tile) // score swap same as placement
					moves = append(moves, move)
				}
			}
		}
	}

	// Sort descending by score
	sort.Slice(moves, func(i, j int) bool {
		return moves[i].Score > moves[j].Score
	})

	return moves
}

func positionalScore(tile, r, c int, minTile, maxTile, BoardSize int, board *Board) float64 {
	// Normalize tile value
	norm := float64(tile-minTile) / float64(maxTile-minTile)

	// Normalize board position
	posWeight := (float64(r) + float64(c)) / float64((BoardSize-1)*2)

	// Alignment score (how well tile fits the position)
	alignment := 1.0 - math.Abs(norm-posWeight)

	// Feasibility: reward cells that leave row/col flexible
	feasibility := estimateFutureFlexibility(board, r, c, tile)

	return alignment * feasibility
}

// estimateFutureFlexibility returns a value 0..1 based on how many legal options
// remain in the row and column after placing tile.
func estimateFutureFlexibility(board *Board, r, c, tile int) float64 {
	BoardSize := len(board.Grid)
	count := 0

	// Check row and column constraints
	for i := 0; i < BoardSize; i++ {
		if i != c && board.Grid[r][i] == 0 {
			count++
		}
		if i != r && board.Grid[i][c] == 0 {
			count++
		}
	}

	return 0.5 + 0.5*float64(count)/float64((BoardSize-1)*2) // normalize to 0.5..1
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

func (state *GameState) applyMove(current int, move Move, tile int) bool {
	board := state.Boards[current]
	switch move.Type {
	case Place:
		board.Grid[move.Cell.R][move.Cell.C] = tile
	case Swap:
		old := board.Grid[move.Cell.R][move.Cell.C]
		board.Grid[move.Cell.R][move.Cell.C] = tile
		state.Table = append(state.Table, old)
	case Discard:
		state.Table = append(state.Table, tile)
		fmt.Println("Discarded to table.")
		return false
	}

	return state.BrunoVariant && checkBrunoExtra(board, move.Cell.R, move.Cell.C)
}

func (state *GameState) PrettyPrintBoardsGridCentered() {
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
		if state.Boards[i].IsAi {
			header = fmt.Sprintf("Computer %d", i)
		}
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

func (state *GameState) initDrawStack() {
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

	for i := 1; i <= 20; i++ {
		if !used[i] {
			state.Draw = append(state.Draw, i)
		}
	}
	rand.Shuffle(len(state.Draw), func(i, j int) {
		state.Draw[i], state.Draw[j] = state.Draw[j], state.Draw[i]
	})
}

func fillRandomDiagonal(b *Board) {
	available := rand.Perm(20)
	for i := 0; i < BoardSize; i++ {
		b.Grid[i][i] = available[i] + 1
	}
}

func (state *GameState) setUpBoards() {
	// --- Ask number of human and AI players ---
	numHumans := 0
	fmt.Print("Number of human players (1-4, default 1): ")
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line != "" {
		n, err := strconv.Atoi(line)
		if err == nil && n >= 1 && n <= 4 {
			numHumans = n
		} else {
			numHumans = 1
		}
	} else {
		numHumans = 1
	}

	numAI := 0
	fmt.Print("Number of computer players (0-4, default 1): ")
	line, _ = reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line != "" {
		n, err := strconv.Atoi(line)
		if err == nil && n >= 0 && n <= 4 {
			numAI = n
		} else {
			numAI = 1
		}
	} else {
		numAI = 1
	}

	totalPlayers := numHumans + numAI
	if totalPlayers > 4 {
		fmt.Println("Max players is 4 — adjusting to 4")
		totalPlayers = 4
		if numAI > 4-numHumans {
			numAI = 4 - numHumans
		}
	}
	if !state.Analyze {
		state.initDrawStack()
	}
	// --- Set up boards ---
	for p := 0; p < totalPlayers; p++ {
		b := &Board{}

		// Assign AI flag
		if p >= numHumans {
			b.IsAi = true
			fmt.Printf("Computer %d board initialized.\n", p-numHumans+1)
		} else {
			b.IsAi = false
			fmt.Printf("Player %d board initialized.\n", p+1)
		}

		// Diagonal setup
		if state.Analyze {
			fmt.Printf("Enter 4 numbers for %s diagonal positions (or leave blank for random): ",
				map[bool]string{true: "Computer", false: "Player"}[b.IsAi])
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input == "" {
				fillRandomDiagonal(b)
			} else {
				nums := strings.Fields(input)
				for i := 0; i < BoardSize && i < len(nums); i++ {
					t, err := strconv.Atoi(nums[i])
					if err == nil {
						b.Grid[i][i] = t
					}
				}
			}
		} else {
			fillRandomDiagonal(b)
		}

		state.Boards = append(state.Boards, b)
	}
}

func promptBrunoVariant() bool {
	fmt.Print("Enable Bruno variant? (extra turn for adjacent diagonal match) (y/N): ")
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}

func (state *GameState) playGame() {
	for {
		board := state.Boards[state.Current]

		var tile int
		if board.IsAi {
			tile, _ = state.drawTileRecommendation()
		} else {
			var quit bool
			tile, quit = state.promptDrawOrSave()
			if quit || tile == -1 {
				fmt.Println("Exiting game.")
				return
			}
		}

		// Unified placement for both human and AI
		state.promptPlacement(state.Current, tile)

		state.PrettyPrintBoardsGridCentered()
		state.Current = (state.Current + 1) % len(state.Boards)
	}
}

func checkBrunoExtra(board *Board, r, c int) bool {
	tile := board.Grid[r][c]
	if tile == 0 {
		return false
	}

	deltas := [][2]int{
		{-1, -1}, {-1, 1}, {1, -1}, {1, 1},
	}

	for _, d := range deltas {
		nr, nc := r+d[0], c+d[1]
		if nr >= 0 && nr < BoardSize && nc >= 0 && nc < BoardSize {
			if board.Grid[nr][nc] == tile {
				fmt.Printf("Bruno’s Variant: matching diagonal at (%d,%d)! Extra turn granted.\n", nr, nc)
				return true
			}
		}
	}
	return false
}

func (state *GameState) promptPlacement(current, tile int) {
	board := state.Boards[current]

	// --- AI-controlled board auto-play ---
	if board.IsAi {
		recs := state.bestMoves(board, tile)
		if len(recs) == 0 {
			// No legal moves, discard to table
			state.applyMove(current, Move{Type: Discard}, tile)
			fmt.Printf("Computer %d discards %d to table.\n", current, tile)
			return
		}

		// Pick best move
		move := recs[0]
		old := board.Grid[move.Cell.R][move.Cell.C]
		extra := state.applyMove(current, move, tile)

		if move.Type == Swap && old != 0 {
			fmt.Printf("Computer %d swaps %d into table, places %d at (%d,%d).\n",
				current, old, tile, move.Cell.R, move.Cell.C)
		} else {
			fmt.Printf("Computer %d places %d at (%d,%d).\n",
				current, tile, move.Cell.R, move.Cell.C)
		}

		if extra {
			fmt.Println("Computer gets extra turn!")
			tile, _ = state.drawTileRecommendation()
			state.promptPlacement(current, tile)
		}

		return
	}

	// --- Human player flow continues unchanged ---
	for {
		fmt.Printf("Action for %d? ([r]ecommend, [t]able, or row,col): ", tile)
		action, _ := reader.ReadString('\n')
		action = strings.TrimSpace(action)

		switch action {
		case "t":
			move := Move{Type: Discard}
			state.applyMove(current, move, tile)
			fmt.Println("Placed on table.")
			return
		case "r":
			recs := state.bestMoves(board, tile)
			if len(recs) == 0 {
				fmt.Println("No legal placements found.")
				continue
			}
			for i, m := range recs {
				fmt.Printf("%d) %s at (%d,%d) — score %.2f\n",
					i+1,
					map[MoveType]string{Place: "Place", Swap: "Swap"}[m.Type],
					m.Cell.R, m.Cell.C, m.Score)
			}
			fmt.Print("Choose move number or press Enter to skip: ")
			choice, _ := reader.ReadString('\n')
			choice = strings.TrimSpace(choice)
			if choice == "" {
				continue
			}
			idx, err := strconv.Atoi(choice)
			if err == nil && idx >= 1 && idx <= len(recs) {
				extra := state.applyMove(current, recs[idx-1], tile)
				if extra {
					continue
				}
				return
			}
			fmt.Println("Invalid choice.")
		default:
			parts := strings.Split(action, ",")
			if len(parts) == 2 {
				r, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
				c, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
				if err1 == nil && err2 == nil && r >= 0 && r < BoardSize && c >= 0 && c < BoardSize {
					if isLegalPlacement(board, tile, r, c) {
						move := Move{Type: Place, Cell: &Cell{R: r, C: c}}
						old := board.Grid[r][c]
						extra := state.applyMove(current, move, tile)
						if old != 0 {
							state.Table = append(state.Table, old)
							fmt.Printf("Swapped %d into table, placed %d at (%d,%d).\n", old, tile, r, c)
						} else {
							fmt.Printf("Placed %d at (%d,%d).\n", tile, r, c)
						}
						if extra {
							continue
						}
						return
					}
				}
			}
			fmt.Println("Invalid input, try again.")
		}
	}
}

func (state *GameState) promptDrawOrSave() (int, bool) {
	for {
		fmt.Print("[d]raw, [r]ecommend, [s]ave, or [q]uit? ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(strings.ToLower(line))

		switch line {
		case "q":
			return 0, true
		case "s":
			fmt.Println("enter file name for save")
			line, _ := reader.ReadString('\n')
			filename := strings.TrimSpace(strings.ToLower(line))
			if !strings.HasSuffix(filename, ".csv") {
				filename += ".csv"
			}
			if err := state.saveToCSV(filename); err != nil {
				fmt.Println("Failed to save:", err)
			} else {
				fmt.Println("Game saved.")
			}
			return -1, false
		case "d", "":
			if state.Analyze {
				fmt.Print("Enter drawn tile: ")
				text, _ := reader.ReadString('\n')
				text = strings.TrimSpace(text)
				if text == "" {
					return 0, true
				}
				tile, err := strconv.Atoi(text)
				if err != nil {
					fmt.Println("Invalid tile number.")
					continue
				}
				return tile, false
			}
			return state.drawTile(), false
		case "r":
			tile, fromTable := state.drawTileRecommendation()
			if fromTable {
				fmt.Printf("Drawing tile %d from the table is the best choice\n", tile)
			}
		default:
			fmt.Println("Invalid option.")
		}
	}
}

func (state *GameState) drawTileRecommendation() (tile int, fromTable bool) {
	board := state.Boards[state.Current]

	bestScore := -1.0
	bestTile := 0
	bestFromTable := false
	bestIndex := 0
	// --- check table tiles ---
	for i, t := range state.Table {
		moves := state.bestMoves(board, t)
		if len(moves) == 0 {
			continue
		}
		score := moves[0].Score // pick top move's score
		if score > bestScore {
			bestScore = score
			bestTile = t
			bestFromTable = true
			bestIndex = i
		}
	}

	// --- check top of draw pile ---
	if len(state.Draw) > 0 {
		t := state.Draw[0]
		moves := state.bestMoves(board, t)
		if len(moves) > 0 && moves[0].Score > bestScore {
			bestScore = moves[0].Score
			bestTile = t
			bestFromTable = false
		}
	}

	// --- draw the chosen tile ---
	if bestTile == 0 {
		// No legal moves anywhere, just take top of draw pile or table
		if len(state.Draw) > 0 {
			bestTile = state.Draw[0]
			bestFromTable = false
		} else if len(state.Table) > 0 {
			bestTile = state.Table[0]
			bestFromTable = true
		} else {
			fmt.Println("No tiles to draw — game over.")
			os.Exit(0)
		}
	}

	if board.IsAi {
		if bestFromTable {
			state.Table = append(state.Table[:bestIndex], state.Table[bestIndex+1:]...)
			fmt.Printf("Computer %d draws %d from table\n", state.Current, bestTile)
		} else {
			state.Draw = state.Draw[1:]
			fmt.Printf("Computer %d draws %d from pile\n", state.Current, bestTile)
		}
	}

	return bestTile, bestFromTable
}

func (state *GameState) drawTile() int {
	if len(state.Table) > 0 {
		fmt.Print("Draw from [p]ile or [t]able? (default pile): ")
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(strings.ToLower(choice))
		if choice == "t" {
			fmt.Println("Tiles on table:", state.Table)
			fmt.Print("Enter tile to pick: ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			tile, err := strconv.Atoi(input)
			if err != nil || !contains(state.Table, tile) {
				fmt.Println("Invalid choice.")
				return state.drawTile()
			}
			for i, v := range state.Table {
				if v == tile {
					state.Table = append(state.Table[:i], state.Table[i+1:]...)
					break
				}
			}
			return tile
		}
	}
	if len(state.Draw) == 0 {
		fmt.Println("Draw pile is empty — game over.")
		os.Exit(0)
	}
	tile := state.Draw[0]
	state.Draw = state.Draw[1:]
	fmt.Printf(" drew a %d\n", tile)
	return tile
}

func (state *GameState) saveToCSV(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	// Write turn info
	writer.Write([]string{"TURN", strconv.Itoa(state.Current)})

	// Write table
	tableRow := []string{"TABLE"}
	for _, t := range state.Table {
		tableRow = append(tableRow, strconv.Itoa(t))
	}
	writer.Write(tableRow)

	// Write boards
	for _, board := range state.Boards {
		for r := 0; r < BoardSize; r++ {
			row := make([]string, BoardSize)
			for c := 0; c < BoardSize; c++ {
				if board.Grid[r][c] == 0 {
					row[c] = "."
				} else {
					row[c] = strconv.Itoa(board.Grid[r][c])
				}
			}
			writer.Write(row)
		}
	}

	return nil
}

func (state *GameState) loadFromCSV(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return err
	}

	if len(records) < 2 {
		return fmt.Errorf("CSV too short")
	}

	state.Boards = []*Board{}
	state.Table = []int{}
	state.Current = 0

	usedTiles := map[int]bool{}

	// --- Parse turn ---
	if records[0][0] != "TURN" {
		return fmt.Errorf("expected TURN record")
	}
	if len(records[0]) < 2 {
		return fmt.Errorf("TURN record missing player index")
	}
	cur, err := strconv.Atoi(records[0][1])
	if err != nil {
		return err
	}
	state.Current = cur
	// --- Parse table ---
	if records[1][0] != "TABLE" {
		return fmt.Errorf("expected TABLE record")
	}
	for _, t := range records[1][1:] {
		if t == "." {
			continue
		}
		n, err := strconv.Atoi(t)
		if err != nil {
			return err
		}
		state.Table = append(state.Table, n)
		usedTiles[n] = true
	}

	// --- Parse boards ---
	var currentBoard *Board
	rowCounter := 0
	for _, rec := range records[2:] {
		if len(rec) != BoardSize {
			return fmt.Errorf("board row with wrong number of fields")
		}
		if rowCounter == 0 {
			currentBoard = &Board{}
		}
		for c, val := range rec {
			if val == "." {
				currentBoard.Grid[rowCounter][c] = 0
			} else {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				currentBoard.Grid[rowCounter][c] = n
				usedTiles[n] = true
			}
		}
		rowCounter++
		if rowCounter == BoardSize {
			state.Boards = append(state.Boards, currentBoard)
			rowCounter = 0
		}
	}

	// --- Generate draw pile ---
	tileCounts := 20 * len(state.Boards) // adjust if using duplicates or more players
	remaining := []int{}
	for i := 1; i <= tileCounts; i++ {
		if !usedTiles[i] {
			remaining = append(remaining, i)
		}
	}
	rand.Shuffle(len(remaining), func(i, j int) { remaining[i], remaining[j] = remaining[j], remaining[i] })
	state.Draw = remaining

	return nil
}

func main() {
	rand.Seed(time.Now().UnixNano())

	fmt.Print("Load from CSV file? (filename or blank for new game): ")
	csvFile, _ := reader.ReadString('\n')
	csvFile = strings.TrimSpace(csvFile)

	state := &GameState{}

	if csvFile != "" {
		if err := state.loadFromCSV(csvFile); err != nil {
			fmt.Println("Failed to load:", err)
			return
		}
		fmt.Println("Loaded game from", csvFile)
	} else {
		fmt.Print("Play or Analyze? (p/a): ")
		mode, _ := reader.ReadString('\n')
		mode = strings.TrimSpace(strings.ToLower(mode))
		if mode == "a" || mode == "analyze" {
			state.Analyze = true
			fmt.Println("Analyze mode selected — manual board setup enabled.")
		} else {
			state.Analyze = false
			fmt.Println("Play mode selected — automatic setup and draw pile enabled.")
		}

		state.setUpBoards()
	}
	state.BrunoVariant = promptBrunoVariant()
	state.PrettyPrintBoardsGridCentered()
	state.playGame()

}
