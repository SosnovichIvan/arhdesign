from pathlib import Path
import sys

from reportlab import rl_config
from reportlab.lib import colors
from reportlab.lib.enums import TA_CENTER, TA_LEFT
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import mm
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import HRFlowable, ListFlowable, ListItem, Paragraph, SimpleDocTemplate, Spacer


VERSION = "2026-09-19-v2"
OUTPUT = Path(sys.argv[1]) if len(sys.argv) > 1 else Path("public/documents/personal-data-policy-2026-09-19-v2.pdf")
FONT_REGULAR = Path("/System/Library/Fonts/Supplemental/Arial.ttf")
FONT_BOLD = Path("/System/Library/Fonts/Supplemental/Arial Bold.ttf")
rl_config.invariant = True


def register_fonts() -> None:
    if not FONT_REGULAR.exists() or not FONT_BOLD.exists():
        raise SystemExit("Arial fonts are required to render Cyrillic text")
    pdfmetrics.registerFont(TTFont("PolicyArial", str(FONT_REGULAR)))
    pdfmetrics.registerFont(TTFont("PolicyArial-Bold", str(FONT_BOLD)))


def footer(canvas, document) -> None:
    canvas.saveState()
    canvas.setStrokeColor(colors.HexColor("#D7D8D3"))
    canvas.setLineWidth(0.5)
    canvas.line(20 * mm, 15 * mm, 190 * mm, 15 * mm)
    canvas.setFont("PolicyArial", 8)
    canvas.setFillColor(colors.HexColor("#6C706A"))
    canvas.drawString(20 * mm, 9 * mm, "designer-svetlana.ru - политика обработки персональных данных")
    canvas.drawRightString(190 * mm, 9 * mm, f"стр. {document.page}")
    canvas.restoreState()


def bullet_list(items, style):
    return ListFlowable(
        [ListItem(Paragraph(item, style), leftIndent=4 * mm) for item in items],
        bulletType="bullet",
        start="circle",
        bulletFontName="PolicyArial",
        bulletFontSize=7,
        leftIndent=7 * mm,
        bulletOffsetY=1.5,
        spaceAfter=4,
    )


def build_document(output: Path) -> None:
    register_fonts()
    output.parent.mkdir(parents=True, exist_ok=True)

    styles = getSampleStyleSheet()
    title = ParagraphStyle(
        "PolicyTitle",
        parent=styles["Title"],
        fontName="PolicyArial-Bold",
        fontSize=19,
        leading=24,
        alignment=TA_CENTER,
        textColor=colors.HexColor("#242B2A"),
        spaceAfter=5 * mm,
    )
    subtitle = ParagraphStyle(
        "PolicySubtitle",
        parent=styles["Normal"],
        fontName="PolicyArial",
        fontSize=9,
        leading=13,
        alignment=TA_CENTER,
        textColor=colors.HexColor("#6C706A"),
        spaceAfter=8 * mm,
    )
    heading = ParagraphStyle(
        "PolicyHeading",
        parent=styles["Heading2"],
        fontName="PolicyArial-Bold",
        fontSize=12,
        leading=16,
        textColor=colors.HexColor("#242B2A"),
        spaceBefore=4 * mm,
        spaceAfter=2 * mm,
        keepWithNext=True,
    )
    body = ParagraphStyle(
        "PolicyBody",
        parent=styles["BodyText"],
        fontName="PolicyArial",
        fontSize=9.6,
        leading=14.5,
        textColor=colors.HexColor("#303532"),
        alignment=TA_LEFT,
        spaceAfter=3 * mm,
    )
    callout = ParagraphStyle(
        "PolicyCallout",
        parent=body,
        backColor=colors.HexColor("#F2F4F0"),
        borderColor=colors.HexColor("#C6D6D0"),
        borderWidth=0.7,
        borderPadding=4 * mm,
        spaceBefore=2 * mm,
        spaceAfter=4 * mm,
    )

    document = SimpleDocTemplate(
        str(output),
        pagesize=A4,
        rightMargin=20 * mm,
        leftMargin=20 * mm,
        topMargin=18 * mm,
        bottomMargin=22 * mm,
        title="Политика обработки персональных данных",
        author="ИП Полисмакова Светлана Александровна",
        subject="Политика сайта designer-svetlana.ru",
    )

    story = [
        Spacer(1, 4 * mm),
        Paragraph("ПОЛИТИКА ОБРАБОТКИ<br/>ПЕРСОНАЛЬНЫХ ДАННЫХ", title),
        Paragraph(f"Сайт designer-svetlana.ru<br/>Версия документа: {VERSION}", subtitle),
        HRFlowable(width="100%", thickness=0.8, color=colors.HexColor("#C6D6D0"), spaceAfter=5 * mm),
        Paragraph("1. Общие положения и оператор", heading),
        Paragraph(
            "Настоящая Политика определяет порядок обработки и защиты персональных данных посетителей "
            "сайта designer-svetlana.ru. Оператор: <b>Индивидуальный предприниматель ПОЛИСМАКОВА "
            "СВЕТЛАНА АЛЕКСАНДРОВНА</b>, сокращенное наименование: <b>ИП ПОЛИСМАКОВА С. А.</b>, "
            "ИНН <b>682962659118</b>, ОГРНИП <b>324508100255269</b>. Почтовый адрес: <b>142000, "
            "Московская область, город Домодедово, Южный внутригородской район, улица Курыжова, "
            "25, 157</b>. Электронная почта: <b>svetlana@polismakova.ru</b>, телефон: "
            "<b>+7 993 335-37-75</b>.",
            body,
        ),
        Paragraph(
            "Политика применяется к данным посетителей, отправляющих форму обратной связи, и к "
            "техническим данным, обрабатываемым при использовании сайта.",
            body,
        ),
        Paragraph("2. Цели, состав данных и правовое основание", heading),
        Paragraph(
            "Правовым основанием является согласие посетителя на обработку персональных данных. "
            "Данные используются для рассмотрения обращения, связи с заявителем, уточнения параметров "
            "проекта, подготовки предложения, доставки служебного уведомления оператору, "
            "предотвращения спама и обеспечения безопасности сайта.",
            body,
        ),
        Paragraph("Обрабатываются:", body),
        bullet_list(
            [
                "имя;",
                "номер телефона или адрес электронной почты;",
                "тип проекта и описание проекта, если оно заполнено;",
                "дата и время отправки, версия, путь и SHA-256 согласия;",
                "адрес страницы, с которой отправлена форма;",
                "IP-адрес, временно используемый в памяти сервера для ограничения частоты запросов;",
                "cookie contact_cooldown и его HMAC-хеш для защиты от повторной отправки.",
            ],
            body,
        ),
        Spacer(1, 3 * mm),
        Paragraph(
            "Сайт не собирает специальные категории, биометрические, паспортные или платежные данные "
            "и не принимает решений, порождающих юридические последствия, исключительно на основании "
            "автоматизированной обработки.",
            body,
        ),
        Paragraph("3. Способы и действия обработки", heading),
        Paragraph(
            "Обработка является автоматизированной и выполняется с передачей по сети Интернет. "
            "Оператор осуществляет сбор, запись, систематизацию, накопление, хранение, уточнение, "
            "извлечение, использование, предоставление доступа, передачу по поручению оператора, "
            "блокирование, удаление и уничтожение данных.",
            body,
        ),
        Paragraph("4. Хранение, инфраструктура и получатели", heading),
        bullet_list(
            [
                "основная база PostgreSQL размещена на VPS REG.RU в Российской Федерации; заявки хранятся до достижения цели, отзыва согласия или не более 365 дней;",
                "Cloudflare Workers используется как технический канал доставки уведомления без намеренного хранения текста заявки;",
                "Telegram Bot API доставляет уведомление в авторизованный чат оператора; сообщение с данными ставится на автоматическое удаление не позднее чем через 24 часа;",
                "резервные копии PostgreSQL создаются ежедневно и автоматически удаляются через 30 дней;",
                "access-логи Caddy хранятся с ротацией не более 30 дней; приложение не записывает содержимое формы в журналы.",
            ],
            body,
        ),
        Spacer(1, 3 * mm),
        Paragraph(
            "Сначала данные записываются в базу на территории Российской Федерации. Затем для доставки "
            "уведомления они передаются через Cloudflare Workers в Telegram Bot API. Такая передача "
            "может осуществляться с использованием инфраструктуры за пределами Российской Федерации "
            "и рассматривается оператором как трансграничная.",
            callout,
        ),
        Paragraph(
            "При включении дополнительного почтового канала оператор до его запуска обновляет Политику "
            "и указывает соответствующего получателя или поставщика.",
            body,
        ),
        Paragraph("5. Cookie и защита от злоупотреблений", heading),
        Paragraph(
            "После принятия заявки сайт устанавливает обязательный cookie contact_cooldown сроком на "
            "один час. Он содержит случайный технический идентификатор, недоступен JavaScript и не "
            "используется для рекламы или аналитики. В PostgreSQL хранится только HMAC-хеш; просроченные "
            "записи удаляются автоматически.",
            body,
        ),
        Paragraph("6. Права посетителя и отзыв согласия", heading),
        Paragraph(
            "Посетитель вправе запросить сведения об обработке, уточнение, блокирование или удаление "
            "данных, отозвать согласие, а также обжаловать действия оператора в Роскомнадзор или в суд. "
            "Запрос направляется на <b>svetlana@polismakova.ru</b> или по почтовому адресу оператора. "
            "В запросе следует указать имя, использованный контакт и суть требования, чтобы оператор "
            "мог найти соответствующую заявку.",
            body,
        ),
        Paragraph("7. Безопасность и уничтожение", heading),
        Paragraph(
            "Оператор применяет разграничение доступа, TLS, изоляцию базы данных во внутренней сети "
            "Docker, секреты окружения, HMAC-хеширование технического идентификатора, ограничение "
            "частоты запросов, журналирование без содержимого формы и автоматическую очистку. После "
            "наступления срока или законного требования данные удаляются из основной базы и связанных "
            "записей; остаточные копии прекращают существование по циклу ротации резервных копий и логов.",
            body,
        ),
        Paragraph("8. Изменение Политики", heading),
        Paragraph(
            "При изменении целей, состава данных, получателей или сроков оператор публикует новую "
            "версию. К заявке привязывается редакция согласия, действовавшая в момент отправки формы. "
            "Действующая версия Политики опубликована по адресу "
            "<b>https://designer-svetlana.ru/privacy</b>.",
            body,
        ),
    ]
    document.build(story, onFirstPage=footer, onLaterPages=footer)


if __name__ == "__main__":
    build_document(OUTPUT)
    print(OUTPUT)
