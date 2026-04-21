package auth

import (
	"context"
	"testing"
)

// mockTeamStore 是 TeamStore 的内存模拟实现。
type mockTeamStore struct {
	teams map[string]*Team
	next  int
}

func newMockTeamStore() *mockTeamStore {
	return &mockTeamStore{teams: make(map[string]*Team)}
}

func (m *mockTeamStore) CreateTeam(_ context.Context, t *Team) (string, error) {
	t.ResID = "team_id_" + string(rune('a'+m.next))
	m.next++
	t2 := *t
	m.teams[t2.ResID] = &t2
	return t2.ResID, nil
}

func (m *mockTeamStore) GetTeamByID(_ context.Context, resID string) (*Team, error) {
	t, ok := m.teams[resID]
	if !ok {
		return nil, nil
	}
	return t, nil
}

func (m *mockTeamStore) ListTeams(_ context.Context) ([]Team, error) {
	var result []Team
	for _, t := range m.teams {
		result = append(result, *t)
	}
	return result, nil
}

func (m *mockTeamStore) UpdateTeam(_ context.Context, t *Team) error {
	if _, ok := m.teams[t.ResID]; !ok {
		return nil
	}
	m.teams[t.ResID] = t
	return nil
}

func (m *mockTeamStore) DeleteTeam(_ context.Context, resID string) error {
	delete(m.teams, resID)
	return nil
}

// mockTeamMemberStore 是 TeamMemberStore 的内存模拟实现。
type mockTeamMemberStore struct {
	members map[string]*TeamMember // key: teamID + ":" + userID
	byTeam  map[string][]*TeamMember
	byUser  map[string][]*TeamMember
	nextID  int
}

func newMockTeamMemberStore() *mockTeamMemberStore {
	return &mockTeamMemberStore{
		members: make(map[string]*TeamMember),
		byTeam:  make(map[string][]*TeamMember),
		byUser:  make(map[string][]*TeamMember),
	}
}

func (m *mockTeamMemberStore) key(teamID, userID string) string {
	return teamID + ":" + userID
}

func (m *mockTeamMemberStore) AddMember(_ context.Context, teamID, userID, role string) (*TeamMember, error) {
	key := m.key(teamID, userID)
	if _, ok := m.members[key]; ok {
		// 检查是否是软删除的，如果是则先移除
		for i, tm := range m.byTeam[teamID] {
			if tm.UserID == userID && tm.IsDeleted {
				m.byTeam[teamID] = append(m.byTeam[teamID][:i], m.byTeam[teamID][i+1:]...)
				break
			}
		}
		for i, tm := range m.byUser[userID] {
			if tm.TeamID == teamID && tm.IsDeleted {
				m.byUser[userID] = append(m.byUser[userID][:i], m.byUser[userID][i+1:]...)
				break
			}
		}
	}

	tm := &TeamMember{
		TeamID: teamID,
		UserID: userID,
		Role:   role,
	}
	tm.ResID = "tm_id_" + string(rune('a'+m.nextID))
	m.nextID++
	m.members[key] = tm
	m.byTeam[teamID] = append(m.byTeam[teamID], tm)
	m.byUser[userID] = append(m.byUser[userID], tm)
	return tm, nil
}

func (m *mockTeamMemberStore) RemoveMember(_ context.Context, teamID, userID string) error {
	key := m.key(teamID, userID)
	if tm, ok := m.members[key]; ok {
		tm.IsDeleted = true
	}
	return nil
}

func (m *mockTeamMemberStore) GetMember(_ context.Context, teamID, userID string) (*TeamMember, error) {
	key := m.key(teamID, userID)
	if tm, ok := m.members[key]; ok && !tm.IsDeleted {
		return tm, nil
	}
	return nil, nil
}

func (m *mockTeamMemberStore) ListTeamMembers(_ context.Context, teamID string) ([]*TeamMember, error) {
	var result []*TeamMember
	for _, tm := range m.byTeam[teamID] {
		if !tm.IsDeleted {
			result = append(result, tm)
		}
	}
	return result, nil
}

func (m *mockTeamMemberStore) GetUserTeams(_ context.Context, userID string) ([]*TeamMember, error) {
	var result []*TeamMember
	for _, tm := range m.byUser[userID] {
		if !tm.IsDeleted {
			result = append(result, tm)
		}
	}
	return result, nil
}

func (m *mockTeamMemberStore) GetTeamMemberCount(_ context.Context, teamID string) (int, error) {
	count := 0
	for _, tm := range m.byTeam[teamID] {
		if !tm.IsDeleted {
			count++
		}
	}
	return count, nil
}

func TestTeamService_CreateTeam(t *testing.T) {
	teamStore := newMockTeamStore()
	userStore := newMockUserStore()
	tmStore := newMockTeamMemberStore()
	svc := NewTeamService(teamStore, userStore, tmStore)

	id, err := svc.CreateTeam(context.Background(), &Team{
		Name:        "Team Alpha",
		Description: "Test team",
	})
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}
	if id == "" {
		t.Error("id should not be empty")
	}
}

func TestTeamService_CreateTeam_EmptyName(t *testing.T) {
	teamStore := newMockTeamStore()
	userStore := newMockUserStore()
	tmStore := newMockTeamMemberStore()
	svc := NewTeamService(teamStore, userStore, tmStore)

	_, err := svc.CreateTeam(context.Background(), &Team{Name: ""})
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestTeamService_GetTeam(t *testing.T) {
	teamStore := newMockTeamStore()
	userStore := newMockUserStore()
	tmStore := newMockTeamMemberStore()
	svc := NewTeamService(teamStore, userStore, tmStore)

	id, _ := svc.CreateTeam(context.Background(), &Team{Name: "Team Alpha"})

	team, err := svc.GetTeam(context.Background(), id)
	if err != nil {
		t.Fatalf("GetTeam: %v", err)
	}
	if team.Name != "Team Alpha" {
		t.Errorf("name = %q, want %q", team.Name, "Team Alpha")
	}
}

func TestTeamService_GetTeam_NotFound(t *testing.T) {
	teamStore := newMockTeamStore()
	userStore := newMockUserStore()
	tmStore := newMockTeamMemberStore()
	svc := NewTeamService(teamStore, userStore, tmStore)

	_, err := svc.GetTeam(context.Background(), "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestTeamService_AddMember(t *testing.T) {
	teamStore := newMockTeamStore()
	userStore := newMockUserStore()
	tmStore := newMockTeamMemberStore()
	svc := NewTeamService(teamStore, userStore, tmStore)

	teamID, _ := svc.CreateTeam(context.Background(), &Team{Name: "Team Alpha"})
	user, _ := newTestAuthService(userStore).Register(context.Background(), &RegisterRequest{
		Username: "player1",
		Password: "pass",
	})

	err := svc.AddMember(context.Background(), teamID, user.ResID, "member")
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	members, _ := svc.ListMembers(context.Background(), teamID)
	if len(members) != 1 {
		t.Fatalf("members count = %d, want 1", len(members))
	}
	if members[0].Username != "player1" {
		t.Errorf("member username = %q, want %q", members[0].Username, "player1")
	}
}

func TestTeamService_RemoveMember(t *testing.T) {
	teamStore := newMockTeamStore()
	userStore := newMockUserStore()
	tmStore := newMockTeamMemberStore()
	svc := NewTeamService(teamStore, userStore, tmStore)

	teamID, _ := svc.CreateTeam(context.Background(), &Team{Name: "Team Alpha"})
	user, _ := newTestAuthService(userStore).Register(context.Background(), &RegisterRequest{
		Username: "player1",
		Password: "pass",
	})

	_ = svc.AddMember(context.Background(), teamID, user.ResID, "member")
	err := svc.RemoveMember(context.Background(), teamID, user.ResID)
	if err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}

	members, _ := svc.ListMembers(context.Background(), teamID)
	if len(members) != 0 {
		t.Errorf("members count = %d, want 0", len(members))
	}
}

func TestTeamService_DeleteTeam(t *testing.T) {
	teamStore := newMockTeamStore()
	userStore := newMockUserStore()
	tmStore := newMockTeamMemberStore()
	svc := NewTeamService(teamStore, userStore, tmStore)

	id, _ := svc.CreateTeam(context.Background(), &Team{Name: "Team Alpha"})
	err := svc.DeleteTeam(context.Background(), id)
	if err != nil {
		t.Fatalf("DeleteTeam: %v", err)
	}

	_, err = svc.GetTeam(context.Background(), id)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}
