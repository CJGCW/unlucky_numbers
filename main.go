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
	Draw
)

type Move struct {
	Type    MoveType
	Cell    *Cell
	Tile    int
	OldTile int     // only set if Type==Swap
	Score   float64 // for ranking which moves are "best"
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
	Current      int
}

var reader = bufio.NewReader(os.Stdin)
var threshold = .50

func (state *GameState) isPlacementFeasible(tile, r, c int) bool {
	remaining := append(state.Draw, state.Table...)
	remainingHigher := []int{}
	remainingLower := []int{}
	for r := 0; r < len(remaining); r++ {
		if tile < remaining[r] {
			remainingHigher = append(remainingHigher, remaining[r])
		}
		if tile > remaining[r] {
			remainingLower = append(remainingLower, remaining[r])
		}
	}
	board := state.Boards[state.Current]
	// Check above
	for rr := r - 1; rr >= 0; rr-- {
		v := board.Grid[rr][c]
		if v == 0 {
			continue
		}
		if v >= tile || (rr < r-1 && !inRange(remainingLower, v, tile)) {
			return false
		}
		break
	}

	// Check left
	for cc := c - 1; cc >= 0; cc-- {
		v := board.Grid[r][cc]
		if v == 0 {
			continue
		}
		if v >= tile || (cc < c-1 && !inRange(remainingLower, v, tile)) {
			return false
		}
		break
	}

	// Check downward feasibility

	for rr := r + 1; rr < BoardSize; rr++ {
		v := board.Grid[rr][c]
		if v != 0 {
			if v <= tile || (rr > r+1 && !inRange(remainingHigher, tile, v)) {
				return false
			}
			break
		}

	}

	// Check rightward feasibility
	for cc := c + 1; cc < BoardSize; cc++ {
		v := board.Grid[r][cc]
		if v != 0 {
			if v <= tile || (cc > c+1 && !inRange(remainingHigher, tile, v)) {
				return false
			}
			break
		}
	}

	return true
}

func (state *GameState) bestMoves(tile int) []Move {
	moves := []Move{}
	board := state.Boards[state.Current]
	for r := 0; r < BoardSize; r++ {
		for c := 0; c < BoardSize; c++ {
			current := board.Grid[r][c]

			feasible := state.isPlacementFeasible(tile, r, c)
			if current == 0 && feasible {
				score := state.placementScore(tile, r, c)
				moves = append(moves, Move{
					Type:  Place,
					Tile:  tile,
					Cell:  &Cell{R: r, C: c},
					Score: score,
				})
			}

			// when the cell has a tile
			if current != 0 && feasible {
				newScore := state.placementScore(tile, r, c)
				oldScore := state.placementScore(current, r, c)

				// Only swap if significant improvement and feasible future
				if newScore > oldScore*1.10 { // at least 10% improvement
					moves = append(moves, Move{
						Type:    Swap,
						Cell:    &Cell{R: r, C: c},
						Tile:    tile,
						OldTile: current,
						Score:   newScore,
					})
				}
			}

		}
	}

	// Sort descending by Score
	sort.SliceStable(moves, func(i, j int) bool {
		return moves[i].Score > moves[j].Score
	})

	return moves
}

// score for how well tile t fits cell (r,c)
func baseScore(tile, r, c int) float64 {
	alpha := 1.00 // this is a score tolerance
	dCell := float64(2 + r + c)
	diff := xOfT(tile) - dCell
	return 100 * math.Exp(-alpha*diff*diff)
}
func xOfT(t int) float64 {
	return 2.0 + 0.32*float64(t-1)
}

func (state *GameState) placementScore(tile, r, c int) float64 {
	base := baseScore(tile, r, c)
	rowProb := state.futureRowProbability(r, c)
	colProb := state.futureColProbability(r, c)
	return base * rowProb * colProb
}

func (state *GameState) printMap(tile int) {
	fmt.Printf("tile %d ", tile)
	fmt.Println("Base score")
	for r := 0; r < BoardSize; r++ {

		for c := 0; c < BoardSize; c++ {
			fmt.Print("| ")
			score := baseScore(tile, r, c)
			fmt.Printf("%5.2f", score)
			fmt.Print("| ")
		}
		fmt.Println()
		fmt.Println("____________________________________")
	}
	fmt.Println("Score considering current tile placements")
	for r := 0; r < BoardSize; r++ {

		for c := 0; c < BoardSize; c++ {
			fmt.Print("| ")
			score := state.placementScore(tile, r, c)
			fmt.Printf("%5.2f", score)
			fmt.Print("| ")
		}
		fmt.Println()
		fmt.Println("____________________________________")
	}
}

// Compute probability row can be filled with unplayed tiles
func (state *GameState) futureRowProbability(r, c int) float64 {
	board := state.Boards[state.Current]
	prob := 1.0
	for cc := 0; cc < BoardSize; cc++ {
		if cc == c || board.Grid[r][cc] != 0 {
			continue
		}
		dist := math.Abs(float64(cc - c))
		weight := 1.0 / (dist + 1.0)

		min, max := state.rowConstraints(r, cc)
		p := state.remainingProbability(min, max)

		prob *= 1 - weight*(1-p)
	}
	return prob
}

// Compute probability row can be filled with unplayed tiles

func (state *GameState) futureColProbability(r, c int) float64 {
	prob := 1.0
	board := state.Boards[state.Current]
	for rr := 0; rr < BoardSize; rr++ {
		if rr == r || board.Grid[rr][c] != 0 {
			continue
		}
		dist := math.Abs(float64(rr - r))
		weight := 1.0 / (dist + 1.0)

		min, max := state.colConstraints(rr, c)
		p := state.remainingProbability(min, max)

		prob *= 1 - weight*(1-p)
	}
	return prob
}

// Compute probability a remaining tile fits in a range
func (state *GameState) remainingProbability(min, max int) float64 {
	if min >= max {
		return 1.0
	}
	total := 0
	count := 0
	for _, t := range append(state.Draw, state.Table...) {
		if t >= min && t <= max {
			count++
		}
		total++
	}
	if total == 0 {
		return 0
	}
	return float64(count) / float64(total)
}

// rowConstraints returns the min/max value that a cell in the row can hold
func (state *GameState) rowConstraints(r, c int) (int, int) {
	min, max := 1, BoardSize*5
	board := state.Boards[state.Current]
	// look left
	for cc := c - 1; cc >= 0; cc-- {
		if board.Grid[r][cc] != 0 {
			min = board.Grid[r][cc] + 1
			break
		}
	}
	// look right
	for cc := c + 1; cc < BoardSize; cc++ {
		if board.Grid[r][cc] != 0 {
			max = board.Grid[r][cc] - 1
			break
		}
	}
	return min, max
}

// colConstraints returns the min/max value that a cell in the column can hold
func (state *GameState) colConstraints(r, c int) (int, int) {
	board := state.Boards[state.Current]
	min, max := 1, BoardSize*5
	// look above
	for rr := r - 1; rr >= 0; rr-- {
		if board.Grid[rr][c] != 0 {
			min = board.Grid[rr][c] + 1
			break
		}
	}
	// look below
	for rr := r + 1; rr < BoardSize; rr++ {
		if board.Grid[rr][c] != 0 {
			max = board.Grid[rr][c] - 1
			break
		}
	}
	return min, max
}

func (state *GameState) applyMove(move Move) bool {
	current := state.Current
	board := state.Boards[current]
	tile := move.Tile
	if board.IsAi {
		prettyType := "nothing?"
		switch move.Type {
		case Place:
			prettyType = "placing"
		case Swap:
			prettyType = "swapping"

		case Discard:
			prettyType = "discarding"
		}
		if move.Type == Discard {
			fmt.Printf("Computer %d is %v tile %d\n", current, prettyType, move.Tile)
		} else {
			fmt.Printf("Computer %d is %v tile %d, (%d,%d)\n", current, prettyType, move.Tile, move.Cell.R, move.Cell.C)
		}
	}
	switch move.Type {
	case Place:
		board.Grid[move.Cell.R][move.Cell.C] = tile
	case Swap:
		old := board.Grid[move.Cell.R][move.Cell.C]
		board.Grid[move.Cell.R][move.Cell.C] = tile
		fmt.Printf("%v to the table\n", old)
		state.Table = append(state.Table, old)
	case Discard:
		state.Table = append(state.Table, tile)
		return false
	}
	if board.IsFull() {
		state.PrettyPrintBoardsGridCentered()
		fmt.Println("GAME OVER!")
		os.Exit(0)
	}
	return state.BrunoVariant && board.checkBrunoExtra(move.Cell.R, move.Cell.C)
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
func inRange(slice []int, lower, higher int) bool {
	for _, v := range slice {
		if v > lower && v < higher {
			return true
		}
	}
	return false
}

func (state *GameState) initDrawStack(totalPlayers int) {
	for b := 1; b <= totalPlayers; b++ {
		for i := 1; i <= 20; i++ {
			state.Draw = append(state.Draw, i)
		}
	}
	rand.Shuffle(len(state.Draw), func(i, j int) {
		state.Draw[i], state.Draw[j] = state.Draw[j], state.Draw[i]
	})
}

func (state *GameState) fillRandomDiagonal(board *Board) {
	for i := 0; i < BoardSize; i++ {
		tile := state.Draw[0]
		state.Draw = state.Draw[1:]
		board.Grid[i][i] = tile
	}
}

func (state *GameState) setUpBoards() {
	// --- Ask number of human and Computer players ---
	numHumans := 1
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
		numHumans = 0
	}

	numAI := 1
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
		numAI = 2
	}

	totalPlayers := numHumans + numAI
	if totalPlayers > 4 {
		fmt.Println("Max players is 4 — adjusting to 4")
		totalPlayers = 4
	}
	if !state.Analyze {
		state.initDrawStack(totalPlayers)
	}
	// --- Set up boards ---
	for p := 0; p < totalPlayers; p++ {
		b := &Board{}

		// Assign Computer flag
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
				state.fillRandomDiagonal(b)
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
			state.fillRandomDiagonal(b)
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

		var move Move
		bestFromTable := false
		if board.IsAi {
			move, bestFromTable = state.drawTileRecommendation()
			if bestFromTable {
				fmt.Printf("Computer is drawing %d from the table\n", move.Tile)
				state.removeTileFromTable(move.Tile)
			} else {

				fmt.Print("Computer draws from pile ")
				move = state.drawTile()
			}
		} else {
			var quit bool
			move, quit = state.promptDrawOrSave()
			if quit {
				fmt.Println("Exiting game.")
				return
			}
		}
		state.promptPlacement(move)

		state.PrettyPrintBoardsGridCentered()
		if board.IsFull() {
			fmt.Println("GAME OVER PG!")
			os.Exit(0)
		}
		state.Current = (state.Current + 1) % len(state.Boards)
	}
}

func (board *Board) checkBrunoExtra(r, c int) bool {
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

func (b *Board) IsFull() bool {
	for r := 0; r < BoardSize; r++ {
		for c := 0; c < BoardSize; c++ {
			if b.Grid[r][c] == 0 {
				return false
			}
		}
	}
	return true
}

func (state *GameState) promptPlacement(move Move) {
	current := state.Current
	board := state.Boards[current]
	tile := move.Tile
	fmt.Printf("Computer %d contemplates %d.\n", current, tile)
	// --- Computer-controlled board auto-play ---
	if board.IsAi {
		if move.Type != Draw {
			state.applyMove(move)
			return
		}
		recs := state.bestMoves(tile)
		if len(recs) == 0 {
			// No legal moves, discard to table
			move.Type = Discard
			state.applyMove(move)
			fmt.Printf("Computer %d discards %d to table.\n", current, tile)
			return
		}
		// Pick best move
		move := recs[0]
		extra := state.applyMove(move)
		if extra {
			fmt.Println("Computer gets extra turn!")
			move, _ = state.drawTileRecommendation()
			state.promptPlacement(move)
		}

		return
	}

	// --- Human player flow continues unchanged ---
	for {
		fmt.Printf("Action for %d? ([r]ecommend, [d]iscard, or row,col): ", tile)
		action, _ := reader.ReadString('\n')
		action = strings.TrimSpace(action)

		switch action {
		case "d":
			move := Move{Type: Discard}
			state.applyMove(move)
			fmt.Println("Placed on table.")
			return
		case "r":
			recs := state.bestMoves(tile)
			if len(recs) == 0 {
				fmt.Println("No legal placements found.")
				continue
			}
			state.printMap(tile)
			for i, m := range recs {
				fmt.Printf("%d) %s at (%d,%d) — score %5.2f\n",
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
				extra := state.applyMove(recs[idx-1])
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
					if state.isPlacementFeasible(tile, r, c) {
						move := Move{Type: Place, Cell: &Cell{R: r, C: c}}
						old := board.Grid[r][c]
						fmt.Println("HOW DID I GET HERE!?")
						extra := state.applyMove(move)
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

func (state *GameState) promptDrawOrSave() (Move, bool) {
	for {
		fmt.Print("[d]raw, [r]ecommend, [s]ave, or [q]uit? ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(strings.ToLower(line))

		switch line {
		case "q":
			return Move{}, true
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
			return Move{}, true
		case "d", "":
			if state.Analyze {
				fmt.Print("Enter drawn tile: ")
				text, _ := reader.ReadString('\n')
				text = strings.TrimSpace(text)
				if text == "" {
					return Move{}, true
				}
				tile, err := strconv.Atoi(text)
				if err != nil {
					fmt.Println("Invalid tile number.")
					continue
				}
				return Move{Tile: tile, Type: Draw}, false
			}
			return state.drawTile(), false
		case "r":
			move, shouldDrawFromTable := state.drawTileRecommendation()
			if !shouldDrawFromTable {
				fmt.Println("Player should draw from the draw stack")
			}

			if move.Type == Swap {
				fmt.Printf("Swapping tile %d from the table into (%d,%d) is the best choice\n", move.Tile, move.Cell.R, move.Cell.C)
			}
			if move.Type == Place {
				fmt.Printf("Placing tile %d from the table into (%d,%d) is the best choice\n", move.Tile, move.Cell.R, move.Cell.C)
			}

		default:
			fmt.Println("Invalid option.")
		}
	}
}

func (state *GameState) drawTileRecommendation() (Move, bool) {

	bestScore := threshold
	bestMove := Move{}
	bestFromTable := false

	for _, t := range state.Table {
		moves := state.bestMoves(t)
		if len(moves) == 0 {
			continue
		}

		score := moves[0].Score

		if score > bestScore {
			bestScore = score

			m := moves[0]
			m.Tile = t
			bestMove = m
			bestFromTable = true
		}
	}

	return bestMove, bestFromTable
}

func (state *GameState) drawTile() Move {
	if !state.Boards[state.Current].IsAi {
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
				state.removeTileFromTable(tile)
				return Move{Tile: tile, Type: Draw}
			}
		}
		if len(state.Draw) == 0 {
			fmt.Println("Draw pile is empty — game over.")
			os.Exit(0)
		}
	}
	tile := state.Draw[0]
	state.Draw = state.Draw[1:]
	fmt.Printf(" drew a %d\n", tile)
	return Move{Tile: tile, Type: Draw}
}
func (state *GameState) removeTileFromTable(tile int) {
	for i, v := range state.Table {
		if v == tile {
			state.Table = append(state.Table[:i], state.Table[i+1:]...)
			break
		}
	}
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
	if csvFile != "" {
		if err := state.loadFromCSV(csvFile); err != nil {
			fmt.Println("Failed to load:", err)
			return
		}
		fmt.Println("Loaded game from", csvFile)
	} else {
		state.setUpBoards()
	}
	state.BrunoVariant = promptBrunoVariant()
	state.PrettyPrintBoardsGridCentered()
	state.playGame()

}
