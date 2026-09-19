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
from reportlab.platypus import (
    HRFlowable,
    KeepTogether,
    ListFlowable,
    ListItem,
    PageBreak,
    Paragraph,
    SimpleDocTemplate,
    Spacer,
)


VERSION = "2026-09-19-v5"
OUTPUT = Path(sys.argv[1]) if len(sys.argv) > 1 else Path("public/documents/personal-data-consent-2026-09-19-v5.pdf")
FONT_REGULAR = Path("/System/Library/Fonts/Supplemental/Arial.ttf")
FONT_BOLD = Path("/System/Library/Fonts/Supplemental/Arial Bold.ttf")
rl_config.invariant = True


def register_fonts() -> None:
    if not FONT_REGULAR.exists() or not FONT_BOLD.exists():
        raise SystemExit("Arial fonts are required to render Cyrillic text")
    pdfmetrics.registerFont(TTFont("ConsentArial", str(FONT_REGULAR)))
    pdfmetrics.registerFont(TTFont("ConsentArial-Bold", str(FONT_BOLD)))


def footer(canvas, document) -> None:
    canvas.saveState()
    canvas.setStrokeColor(colors.HexColor("#D7D8D3"))
    canvas.setLineWidth(0.5)
    canvas.line(20 * mm, 15 * mm, 190 * mm, 15 * mm)
    canvas.setFont("ConsentArial", 8)
    canvas.setFillColor(colors.HexColor("#6C706A"))
    canvas.drawString(20 * mm, 9 * mm, "designer-svetlana.ru - согласие на обработку персональных данных")
    canvas.drawRightString(190 * mm, 9 * mm, f"стр. {document.page}")
    canvas.restoreState()


def bullet_list(items, style):
    return ListFlowable(
        [ListItem(Paragraph(item, style), leftIndent=4 * mm) for item in items],
        bulletType="bullet",
        start="circle",
        bulletFontName="ConsentArial",
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
        "ConsentTitle",
        parent=styles["Title"],
        fontName="ConsentArial-Bold",
        fontSize=20,
        leading=25,
        alignment=TA_CENTER,
        textColor=colors.HexColor("#242B2A"),
        spaceAfter=5 * mm,
    )
    subtitle = ParagraphStyle(
        "ConsentSubtitle",
        parent=styles["Normal"],
        fontName="ConsentArial",
        fontSize=9,
        leading=13,
        alignment=TA_CENTER,
        textColor=colors.HexColor("#6C706A"),
        spaceAfter=8 * mm,
    )
    heading = ParagraphStyle(
        "ConsentHeading",
        parent=styles["Heading2"],
        fontName="ConsentArial-Bold",
        fontSize=12,
        leading=16,
        textColor=colors.HexColor("#242B2A"),
        spaceBefore=4 * mm,
        spaceAfter=2 * mm,
    )
    body = ParagraphStyle(
        "ConsentBody",
        parent=styles["BodyText"],
        fontName="ConsentArial",
        fontSize=10,
        leading=15,
        textColor=colors.HexColor("#303532"),
        alignment=TA_LEFT,
        spaceAfter=3 * mm,
    )
    small = ParagraphStyle(
        "ConsentSmall",
        parent=body,
        fontSize=8.8,
        leading=13,
        textColor=colors.HexColor("#5B615D"),
    )
    callout = ParagraphStyle(
        "ConsentCallout",
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
        title="Согласие на обработку персональных данных",
        author="ИП Полисмакова Светлана Александровна",
        subject="Согласие посетителя сайта designer-svetlana.ru",
    )

    story = [
        Spacer(1, 4 * mm),
        Paragraph("СОГЛАСИЕ НА ОБРАБОТКУ<br/>ПЕРСОНАЛЬНЫХ ДАННЫХ", title),
        Paragraph(f"Сайт designer-svetlana.ru<br/>Версия документа: {VERSION}", subtitle),
        HRFlowable(width="100%", thickness=0.8, color=colors.HexColor("#C6D6D0"), spaceAfter=5 * mm),
        Paragraph("1. Оператор персональных данных", heading),
        Paragraph(
            "Оператор: <b>Индивидуальный предприниматель ПОЛИСМАКОВА СВЕТЛАНА АЛЕКСАНДРОВНА</b>. "
            "Сокращенное наименование: <b>ИП ПОЛИСМАКОВА С. А.</b>. Организационно-правовая "
            "форма: <b>индивидуальный предприниматель</b>. ИНН: <b>682962659118</b>. "
            "ОГРНИП: <b>324508100255269</b>. Почтовый адрес: <b>142000, Московская область, "
            "город Домодедово, Южный внутригородской район, улица Курыжова, 25, 157</b>. "
            "Контактные данные оператора: <b>svetlana@polismakova.ru</b>, <b>+7 993 335-37-75</b>.",
            body,
        ),
        Paragraph("2. Предоставление согласия", heading),
        Paragraph(
            "Посетитель сайта предоставляет согласие добровольно, своей волей и в своем интересе "
            "отдельным действием: устанавливает флажок согласия в форме обратной связи, после чего "
            "отправляет форму. Согласие является конкретным, предметным, информированным, сознательным "
            "и однозначным.",
            body,
        ),
        Paragraph("3. Персональные данные", heading),
        Paragraph("Обрабатываются следующие данные, предоставленные посетителем:", body),
        bullet_list(
            [
                "имя;",
                "номер телефона или адрес электронной почты;",
                "тип проекта;",
                "описание проекта, если оно заполнено посетителем.",
                "дата и время отправки формы, версия и контрольная сумма настоящего документа;",
                "адрес страницы, с которой отправлена форма.",
            ],
            body,
        ),
        Paragraph(
            "Для обеспечения безопасности сайта дополнительно обрабатываются технические сведения "
            "о запросе. IP-адрес используется в памяти сервера для ограничения частоты запросов. "
            "После принятия заявки устанавливается обязательный cookie contact_cooldown сроком на один "
            "час, а в PostgreSQL хранится только его HMAC-хеш. Эти сведения не используются для рекламы.",
            body,
        ),
        Paragraph("4. Цели обработки", heading),
        Paragraph("Данные используются исключительно для следующих целей:", body),
        bullet_list(
            [
                "рассмотрение обращения;",
                "связь с заявителем и уточнение параметров проекта;",
                "подготовка предложения по проектированию или дизайну;",
                "уведомление оператора о новой заявке;",
                "предотвращение спама и обеспечение безопасности сайта.",
            ],
            body,
        ),
        KeepTogether(
            [
                Paragraph("5. Действия с персональными данными", heading),
                Paragraph(
                    "Обработка выполняется автоматизированным способом с передачей по сети Интернет. "
                    "Согласие распространяется на сбор, запись, систематизацию, накопление, хранение, "
                    "уточнение, извлечение, использование, предоставление доступа, передачу по поручению "
                    "оператора, блокирование, удаление и уничтожение персональных данных в объеме, "
                    "необходимом для указанных целей.",
                    body,
                ),
            ]
        ),
        Paragraph("6. Хранение и получатели", heading),
        Paragraph("Для работы сервиса используются следующие поставщики и инфраструктура:", body),
        bullet_list(
            [
                "REG.RU - размещение VPS, основной базы PostgreSQL и резервных копий в Российской Федерации;",
                "Cloudflare Workers - технический канал передачи уведомления без намеренного хранения заявки;",
                "Telegram Bot API - доставка уведомления в авторизованный чат оператора.",
            ],
            body,
        ),
        Paragraph(
            "Заявки хранятся в основной базе PostgreSQL до достижения цели обработки, отзыва согласия "
            "или не более 365 дней. Сообщение Telegram с данными ставится на автоматическое удаление "
            "не позднее чем через 24 часа. Резервные копии базы и access-логи хранятся не более 30 дней. "
            "Приложение не записывает содержимое формы в журналы.",
            body,
        ),
        Paragraph(
            "Сначала данные записываются в базу на территории Российской Федерации. Затем для доставки "
            "уведомления они передаются через Cloudflare Workers в Telegram Bot API. Передача может "
            "осуществляться с использованием инфраструктуры за пределами Российской Федерации и "
            "рассматривается оператором как трансграничная.",
            callout,
        ),
        Paragraph("7. Срок действия и отзыв", heading),
        Paragraph(
            "Согласие действует до достижения целей обработки, его отзыва или истечения срока хранения. "
            "Отзыв согласия, запрос на доступ, исправление или удаление данных можно направить оператору "
            "по адресу <b>svetlana@polismakova.ru</b> или по почтовому адресу оператора, указанному в "
            "разделе 1. В запросе следует указать имя, использованный контакт и суть требования. Отзыв "
            "не влияет на законность обработки, выполненной до его получения.",
            body,
        ),
        Paragraph("8. Ограничения использования", heading),
        Paragraph(
            "Персональные данные не продаются, не публикуются в открытом доступе и не используются "
            "для рекламных рассылок без отдельного согласия. Сайт не запрашивает паспортные данные, "
            "платежные реквизиты, биометрические данные и сведения о здоровье.",
            body,
        ),
        Paragraph("9. Подтверждение", heading),
        Paragraph(
            "Нажимая кнопку отправки формы после установки флажка согласия, посетитель подтверждает, "
            "что ознакомился с настоящим документом и согласен с указанными условиями обработки "
            "персональных данных. Политика обработки персональных данных опубликована по адресу "
            "<b>https://designer-svetlana.ru/privacy</b>.",
            body,
        ),
    ]
    document.build(story, onFirstPage=footer, onLaterPages=footer)


if __name__ == "__main__":
    build_document(OUTPUT)
    print(OUTPUT)
