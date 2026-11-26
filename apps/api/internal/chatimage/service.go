package chatimage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const MaxBytes = 10 * 1024 * 1024
const PageSize = 24

var (
	ErrInvalid   = errors.New("invalid chat image")
	ErrNotFound  = errors.New("chat image or conversation not found")
	ErrForbidden = errors.New("cannot delete this image")
	ErrConflict  = errors.New("image upload key was already used")
)

type Access struct {
	UserID     string
	ChatID     string
	ProjectID  string
	SuperAdmin bool
}

func (access Access) Scope() string {
	if access.ProjectID != "" {
		return "project"
	}
	return "global"
}

type Image struct {
	ID            string
	Name          string
	MediaType     string
	SizeBytes     int64
	Width, Height int
	CreatedAt     time.Time
	Pinned        bool
	CanDelete     bool
}

type Upload struct {
	Image
	Hash    [sha256.Size]byte
	Content []byte
}

type Content struct {
	MediaType string
	Bytes     []byte
}

type Cursor struct {
	Pinned    bool
	CreatedAt time.Time
	ID        string
}

type Store interface {
	ListChatImages(context.Context, Access, *Cursor) ([]Image, error)
	StoreChatImage(context.Context, Access, Upload) (Image, error)
	ViewChatImage(context.Context, Access, string) (Content, error)
	SetChatImagePin(context.Context, Access, string, bool) error
	DeleteChatImage(context.Context, Access, string) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (service *Service) List(ctx context.Context, access Access, cursor string) ([]Image, *string, error) {
	after, err := parseCursor(cursor)
	if err != nil {
		return nil, nil, err
	}
	items, err := service.store.ListChatImages(ctx, access, after)
	if err != nil || len(items) <= PageSize {
		return items, nil, err
	}
	last := items[PageSize-1]
	cursor = base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatBool(last.Pinned) + ":" + strconv.FormatInt(last.CreatedAt.UnixMicro(), 10) + ":" + last.ID))
	return items[:PageSize], &cursor, nil
}

func (service *Service) Upload(ctx context.Context, access Access, id, name string, reader io.Reader) (Image, error) {
	name = strings.TrimSpace(name)
	if id == uuid.Nil.String() || !validName(name) {
		return Image{}, ErrInvalid
	}
	content, err := io.ReadAll(io.LimitReader(reader, MaxBytes+1))
	if err != nil || len(content) == 0 || len(content) > MaxBytes {
		return Image{}, ErrInvalid
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 12000 || config.Height > 12000 || config.Width*config.Height > 40_000_000 {
		return Image{}, ErrInvalid
	}
	mediaType := "image/" + format
	if mediaType != "image/jpeg" && mediaType != "image/png" && mediaType != "image/gif" {
		return Image{}, ErrInvalid
	}
	image := Image{ID: id, Name: name, MediaType: mediaType, SizeBytes: int64(len(content)), Width: config.Width, Height: config.Height}
	return service.store.StoreChatImage(ctx, access, Upload{Image: image, Hash: sha256.Sum256(content), Content: content})
}

func (service *Service) View(ctx context.Context, access Access, id string) (Content, error) {
	return service.store.ViewChatImage(ctx, access, id)
}

func (service *Service) Pin(ctx context.Context, access Access, id string, pinned bool) error {
	return service.store.SetChatImagePin(ctx, access, id, pinned)
}

func (service *Service) Delete(ctx context.Context, access Access, id string) error {
	return service.store.DeleteChatImage(ctx, access, id)
}

func validName(name string) bool {
	length := utf8.RuneCountInString(name)
	return length >= 1 && length <= 240 && utf8.ValidString(name) &&
		!strings.ContainsAny(name, "/\\") && strings.IndexFunc(name, unicode.IsControl) == -1
}

func parseCursor(raw string) (*Cursor, error) {
	if raw == "" {
		return nil, nil
	}
	if len(raw) > 128 {
		return nil, ErrInvalid
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, ErrInvalid
	}
	fields := strings.Split(string(decoded), ":")
	if len(fields) != 3 {
		return nil, ErrInvalid
	}
	pinned, pinErr := strconv.ParseBool(fields[0])
	microseconds, timeErr := strconv.ParseInt(fields[1], 10, 64)
	id, idErr := uuid.Parse(fields[2])
	if pinErr != nil || timeErr != nil || microseconds <= 0 || idErr != nil {
		return nil, ErrInvalid
	}
	return &Cursor{Pinned: pinned, CreatedAt: time.UnixMicro(microseconds).UTC(), ID: id.String()}, nil
}
