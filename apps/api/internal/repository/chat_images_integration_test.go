//go:build integration

package repository

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/chatimage"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/globalchat"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/project"
	"github.com/google/uuid"
)

func TestChatImagesAndConversationVisibilityIntegration(t *testing.T) {
	pool := newMigrationSchemaPool(t, "chat_images")
	paths := migrationPaths(t)
	applyMigrationPaths(t, pool, paths[:len(paths)-1])
	ctx := context.Background()
	store := NewPostgres(pool)
	cipher, err := account.NewPayloadCipher(bytes.Repeat([]byte{3}, 32), 1)
	if err != nil {
		t.Fatal(err)
	}
	owner := seedCollaborationUser(t, pool, "image.owner", "Владелец", nil)
	member := seedCollaborationUser(t, pool, "image.member", "Участник", nil)
	outsider := seedCollaborationUser(t, pool, "image.outsider", "Другой участник проекта", nil)
	role := "technical_admin"
	admin := seedCollaborationUser(t, pool, "image.admin", "Администратор", &role)
	global, err := globalchat.NewService(store, bytes.Repeat([]byte{1}, 32), cipher, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	globalChat, err := global.Create(ctx, globalchat.Actor{UserID: owner}, globalchat.CreateRequest{Kind: "direct", MemberUserIDs: []string{member}, RequestID: "chat-image-global-create"})
	if err != nil {
		t.Fatal(err)
	}
	projects, err := project.NewService(store, bytes.Repeat([]byte{2}, 32), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	ownerActor := project.Actor{UserID: owner}
	projectRecord, err := projects.Create(ctx, ownerActor, project.CreateRequest{Name: "Изображения", Type: "interior_design", CurrencyCode: "RUB", RequestID: "chat-image-project-create"})
	if err != nil {
		t.Fatal(err)
	}
	for _, person := range []string{member, outsider} {
		if _, err := projects.AddMember(ctx, ownerActor, projectRecord.ID, person, "chat-image-project-member"); err != nil {
			t.Fatal(err)
		}
	}
	projectChat, err := projects.CreateChat(ctx, ownerActor, projectRecord.ID, project.CreateChatRequest{Name: "Согласование", MemberUserIDs: []string{member}, RequestID: "chat-image-project-chat"})
	if err != nil {
		t.Fatal(err)
	}
	// The migration is additive and must leave existing chats and membership in place.
	applyMigrationPaths(t, pool, paths[len(paths)-1:])
	for _, scenario := range []struct {
		scope, chatID, projectID string
	}{
		{"global", globalChat.ID, ""},
		{"project", projectChat.ID, projectRecord.ID},
	} {
		t.Run(scenario.scope, func(t *testing.T) {
			if scenario.scope == "global" {
				for _, actor := range []globalchat.Actor{{UserID: outsider}, {UserID: admin}} {
					items, listErr := global.List(ctx, globalchat.ListRequest{Actor: actor, PageSize: 50})
					if listErr != nil || len(items.Items) != 0 {
						t.Fatalf("private global chat list: %+v %v", items, listErr)
					}
					if _, getErr := global.Get(ctx, actor, scenario.chatID, 50, ""); !errors.Is(getErr, globalchat.ErrNotFound) {
						t.Fatalf("global outsider direct access: %v", getErr)
					}
				}
			} else {
				visitor := project.Actor{UserID: outsider}
				manager := project.Actor{UserID: admin, SuperAdmin: true}
				findChat := func(actor project.Actor) bool {
					list, listErr := projects.ListChats(ctx, actor, scenario.projectID)
					if listErr != nil {
						t.Fatal(listErr)
					}
					return slices.ContainsFunc(list.Items, func(v project.ChatSummary) bool { return v.ID == scenario.chatID })
				}
				if findChat(visitor) || !findChat(manager) || !findChat(project.Actor{UserID: member}) {
					t.Fatal("project chat list ignored membership or administrator access")
				}
				if _, getErr := projects.ListContextChat(ctx, visitor, scenario.projectID, scenario.chatID, 50, ""); !errors.Is(getErr, project.ErrNotFound) {
					t.Fatalf("project outsider direct access: %v", getErr)
				}
			}

			images := chatimage.NewService(store)
			location := chatimage.Access{ChatID: scenario.chatID, ProjectID: scenario.projectID, UserID: owner}
			png, decodeErr := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+X2GQAAAAASUVORK5CYII=")
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}
			imageID := uuid.NewString()
			if _, uploadErr := images.Upload(ctx, location, imageID, "wrong.png", bytes.NewBufferString("<html>")); !errors.Is(uploadErr, chatimage.ErrInvalid) {
				t.Fatalf("invalid image upload: %v", uploadErr)
			}
			for range 2 {
				image, uploadErr := images.Upload(ctx, location, imageID, "План.png", bytes.NewReader(png))
				if uploadErr != nil || image.ID != imageID || image.Width != 1 || image.MediaType != "image/png" || !image.CanDelete {
					t.Fatalf("image replay: %+v %v", image, uploadErr)
				}
			}
			if _, uploadErr := images.Upload(ctx, location, imageID, "different.png", bytes.NewReader(png)); !errors.Is(uploadErr, chatimage.ErrConflict) {
				t.Fatalf("upload id conflict: %v", uploadErr)
			}
			other := location
			other.UserID = outsider
			if _, _, listErr := images.List(ctx, other, ""); !errors.Is(listErr, chatimage.ErrNotFound) {
				t.Fatalf("outsider listing: %v", listErr)
			}
			if _, viewErr := images.View(ctx, other, imageID); !errors.Is(viewErr, chatimage.ErrNotFound) {
				t.Fatalf("outsider image access: %v", viewErr)
			}
			if pinErr := images.Pin(ctx, other, imageID, true); !errors.Is(pinErr, chatimage.ErrNotFound) {
				t.Fatalf("outsider pin: %v", pinErr)
			}
			administrator := location
			administrator.UserID, administrator.SuperAdmin = admin, true
			_, viewErr := images.View(ctx, administrator, imageID)
			if (scenario.scope == "global" && !errors.Is(viewErr, chatimage.ErrNotFound)) || (scenario.scope == "project" && viewErr != nil) {
				t.Fatalf("administrator visibility for %s: %v", scenario.scope, viewErr)
			}
			participant := location
			participant.UserID = member
			bytesView, err := images.View(ctx, participant, imageID)
			if err != nil || !bytes.Equal(bytesView.Bytes, png) {
				t.Fatalf("member image access: %v", err)
			}
			if err := images.Delete(ctx, participant, imageID); !errors.Is(err, chatimage.ErrForbidden) {
				t.Fatalf("delete another participant's image: %v", err)
			}
			if err := images.Pin(ctx, participant, imageID, true); err != nil {
				t.Fatal(err)
			}
			items, _, err := images.List(ctx, participant, "")
			if err != nil || len(items) != 1 || !items[0].Pinned || items[0].CanDelete {
				t.Fatalf("pinned list: %+v %v", items, err)
			}
			if err := images.Pin(ctx, participant, imageID, false); err != nil {
				t.Fatal(err)
			}
			if err := images.Delete(ctx, location, imageID); err != nil {
				t.Fatal(err)
			}
			if _, err := images.View(ctx, participant, imageID); !errors.Is(err, chatimage.ErrNotFound) {
				t.Fatalf("view after deletion: %v", err)
			}
			if _, err := images.Upload(ctx, location, imageID, "План.png", bytes.NewReader(png)); !errors.Is(err, chatimage.ErrConflict) {
				t.Fatalf("retry should not recreate deleted image: %v", err)
			}
		})
	}
}
