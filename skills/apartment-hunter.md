---
name: apartment-hunter
description: Apartment search with filtering, comparison, and application generation
version: 1.0.0
metadata:
  openclaw:
    requires:
      env: []
---

# Apartment Hunter

Autonomous apartment search, filtering, and application assistance.

## When to Trigger

- User mentions apartment hunting, moving, or housing search
- Scheduled cron for ongoing apartment monitoring
- User asks about rental market in a city

## Preparation

Recall context before searching:
- D9 (Place): Current location, preferred neighborhoods, commute requirements
- D8 (Finances): Budget, income for affordability calculations
- D11 (Preferences): Must-haves (balcony, pets, quiet), deal-breakers

Use `recall_memory` with keywords: "apartment", "housing", "budget", "neighborhood preferences"

## Search Workflow

### 1. Gather Requirements

If not already known from memory:
- City/neighborhood
- Budget range (warm rent vs cold rent)
- Size (rooms, sqm)
- Move-in date
- Must-haves and nice-to-haves

### 2. Search Sources

Use `search_web` for:
- ImmoScout24 (Germany)
- WG-Gesucht (shared apartments)
- Immowelt
- Local Facebook groups
- City-specific platforms

Search query format: `[city] [rooms]+ zimmer wohnung miete [max budget]`

### 3. Filter Results

Apply user's criteria:
- Budget (include Nebenkosten in total)
- Location (commute time to work if known)
- Size requirements
- Pet policy if relevant
- Available from date

### 4. Present Findings

Format each listing:

```
**[Title]**
- Location: [Neighborhood], [City]
- Rent: [Kaltmiete] + [Nebenkosten] = [Warmmiete]
- Size: [sqm], [rooms] rooms
- Available: [date]
- Highlights: [balcony, renovated, etc.]
- Link: [url]
- Match Score: [%] based on preferences
```

Group by: Best matches, Acceptable, Stretch (over budget or missing features)

## Application Generation (Bewerbung)

When user wants to apply, generate a German rental application letter:

### Required Information

From memory or ask:
- Full name
- Current address
- Occupation and employer
- Monthly net income
- Reason for moving
- Move-in date flexibility

### Letter Template

```
Sehr geehrte Damen und Herren,

mit großem Interesse habe ich Ihre Wohnungsanzeige für die [rooms]-Zimmer-Wohnung in [address/neighborhood] gesehen.

Zu meiner Person: Ich bin [name], [age] Jahre alt, und arbeite als [occupation] bei [employer]. Mein monatliches Nettoeinkommen beträgt [income] EUR.

[Reason for moving / why this apartment suits them]

Ich bin ein zuverlässiger Mieter und kann bei Bedarf folgende Unterlagen vorlegen:
- Gehaltsabrechnungen der letzten drei Monate
- SCHUFA-Auskunft
- Mietschuldenfreiheitsbescheinigung
- Kopie des Personalausweises

Für einen Besichtigungstermin stehe ich flexibel zur Verfügung. Ich freue mich auf Ihre Rückmeldung.

Mit freundlichen Grüßen,
[name]
[phone]
[email]
```

## Ongoing Monitoring

If user wants continuous search:
1. Set up cron with keyword "apartment-search"
2. On trigger, search configured sources
3. Filter for new listings (compare against previously seen)
4. Notify only if good matches found
5. Save seen listings to avoid duplicates

## Guidelines

- Calculate total monthly cost (rent + utilities + internet estimate)
- Flag potential scams (too good to be true, upfront payment requests)
- Note if listing requires quick action (high demand areas)
- Respect user's autonomy: present options, don't pressure
- For German rentals, remind about Anmeldung, Kaution (usually 3 months), and notice periods
