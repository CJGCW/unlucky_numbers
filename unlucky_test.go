package main

import (
	"testing"
)

func exampleStateForTests() *GameState {
	board1 := &Board{
		Grid: [BoardSize][BoardSize]int{
			{5, 0, 0, 9},
			{0, 7, 0, 0},
			{0, 0, 10, 19},
			{0, 0, 19, 20},
		},
	}
	board2 := &Board{
		Grid: [BoardSize][BoardSize]int{
			{6, 0, 0, 0},
			{0, 10, 0, 0},
			{0, 0, 14, 0},
			{0, 0, 0, 20},
		},
	}
	return &GameState{
		Boards: []*Board{board1, board2},
		Table:  []int{7, 5, 17, 4, 10},
		Draw:   []int{},
	}
}

func TestIsLegalPlacement(t *testing.T) {
	state := exampleStateForTests()
	b := state.Boards[0]

	if !isLegalPlacement(b, 6, 0, 1) {
		t.Errorf("Expected 6 to be legal at (0,1)")
	}
	if !isLegalPlacement(b, 8, 2, 1) {
		t.Errorf("Expected 8 to be legal at (2,1)")
	}

	if isLegalPlacement(b, 4, 0, 1) {
		t.Errorf("Expected 4 to be illegal at (0,1)")
	}
	if isLegalPlacement(b, 20, 3, 0) {
		t.Errorf("Expected 20 to be illegal at (3,0)")
	}
}

func TestIsPlacementFeasible(t *testing.T) {
	state := exampleStateForTests()
	b := state.Boards[1]
	remaining := getRemainingTileCounts(state, b)
	PrettyPrintBoardsGridCentered(state.Boards)
	if !isPlacementFeasible(b, 0, 1, 6, remaining) {
		t.Errorf("Expected 6 at (0,1) to be feasible")
	}
	if isPlacementFeasible(b, 0, 3, 18, remaining) {
		t.Errorf("Expected 18 at (0,3) to be infeasible due to missing 19s")
	}
}

func TestGetRemainingTileCounts(t *testing.T) {
	state := exampleStateForTests()
	b := state.Boards[1]
	counts := getRemainingTileCounts(state, b)

	if counts[19] != 0 {
		t.Errorf("Expected remaining count of 19 to be 0, got %d", counts[19])
	}
	if counts[18] == 0 {
		t.Errorf("Expected remaining count of 18 > 0")
	}
}

func TestBestPlacement(t *testing.T) {
	state := exampleStateForTests()
	board := state.Boards[1]

	cell := bestPlacement(board, 18, state)
	if cell == nil {
		t.Errorf("Expected a legal placement for 18")
	} else {
		// Should not pick column 3 because both 19s are gone
		if cell.C == 3 {
			t.Errorf("Placement for 18 should not be in column 3 due to missing 19s")
		}
	}
}

func TestBestMoves(t *testing.T) {
	state := exampleStateForTests()
	board := state.Boards[1]

	moves := bestMoves(board, 18, state, 3)
	if len(moves) == 0 {
		t.Errorf("Expected at least one move for 18")
	}

	for _, m := range moves {
		if m.Type == Place && m.Cell.C == 3 {
			t.Errorf("Placement at column 3 should not be suggested due to missing 19s")
		}
		if m.Type == Swap {
			if m.Cell.C == 3 && m.OldTile != 0 {
				t.Errorf("Swap in column 3 should not be suggested due to missing 19s")
			}
		}
	}
}

func TestSwapLegality(t *testing.T) {
	state := exampleStateForTests()
	board := state.Boards[1]

	moves := bestMoves(board, 18, state, 5)
	swapFound := false
	for _, m := range moves {
		if m.Type == Swap {
			swapFound = true
			if !isLegalPlacement(board, 18, m.Cell.R, m.Cell.C) {
				t.Errorf("Swap suggested at (%d,%d) is illegal", m.Cell.R, m.Cell.C)
			}
		}
	}
	if !swapFound {
		t.Logf("No swaps found — correct if no legal swaps exist")
	}
}
