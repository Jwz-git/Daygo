export default {
  archetype: {
    label: 'Archetype',
    hint: 'Decides which buildings and landscape this category becomes in the 3D world.',
    forge: 'Create',
    sanctum: 'Think',
    nexus: 'Connect',
    camp: 'Rest',
    order: 'Organize',
  },
  profession: {
    softwareEngineer: {
      name: 'Software engineer',
      categories: {
        coding: { name: 'Coding / Debugging', details: 'Writing, refactoring, and fixing code in an IDE or terminal' },
        codeReview: { name: 'Code Review', details: 'Reviewing PRs, reading diffs, and leaving comments' },
        research: { name: 'Research', details: 'Reading docs, Stack Overflow, exploring tools and APIs, and writing design docs or technical specs' },
        communication: { name: 'Communication', details: 'Meetings, standups, Slack, email, video calls, messaging, and syncs' },
      },
    },
    founderExecutive: {
      name: 'Founder / executive',
      categories: {
        building: { name: 'Engineering / Product', details: 'Coding, design work, shipping features, and hands-on building' },
        strategy: { name: 'Research & Strategy', details: 'Competitive research, positioning, long-form thinking, and investor prep' },
        insights: { name: 'Data & Insights', details: 'Dashboards, retention data, funnels, and financials' },
        communication: { name: 'Communication', details: 'Team syncs, investor calls, user demos, and hiring' },
      },
    },
    designer: {
      name: 'Designer',
      categories: {
        design: { name: 'Design', details: 'Prototyping, UI components, user flows, visual design, and handoff specs' },
        research: { name: 'Research', details: 'Browsing patterns, competitive audits, user studies, and reviewing metrics' },
        communication: { name: 'Communication', details: 'Design reviews, standups, critique sessions, and presenting concepts' },
      },
    },
    student: {
      name: 'Student',
      categories: {
        studying: { name: 'Studying', details: 'Lectures, reading, reviewing slides, flashcards, and course material' },
        assignments: { name: 'Assignments', details: 'Papers, problem sets, coding projects, and lab reports' },
        communication: { name: 'Communication', details: 'Study groups, office hours, group chats, and emailing professors' },
      },
    },
    productManager: {
      name: 'Product manager',
      categories: {
        specs: { name: 'Specs & Planning', details: 'PRDs, roadmaps, backlog grooming, sprint planning, and tickets' },
        research: { name: 'Research & Analysis', details: 'User research, metrics review, competitive analysis, and A/B tests' },
        communication: { name: 'Communication', details: 'Standups, stakeholder syncs, design reviews, and engineering check-ins' },
      },
    },
    dataScientist: {
      name: 'Data scientist',
      categories: {
        analysis: { name: 'Analysis & Modeling', details: 'Notebooks, statistical analysis, ML training, and data exploration' },
        dataEngineering: { name: 'Data Engineering', details: 'SQL queries, pipelines, data cleaning, and ETL scripts' },
        research: { name: 'Research', details: 'Reading papers, docs, and exploring new methods and tools' },
        communication: { name: 'Communication', details: 'Presenting findings, stakeholder syncs, and team discussions' },
      },
    },
    other: {
      name: 'Other',
      categories: {
        work: { name: 'Work', details: 'Focused work tasks and professional responsibilities that do not fit a more specific category' },
        communication: { name: 'Communication', details: 'Meetings, standups, Slack, email, video calls, messaging, and syncs' },
      },
    },
  },
  common: {
    distraction: { name: 'Distraction', details: 'Unfocused browsing and passive content consumption: social media feeds, random videos, idle scrolling, entertainment with no clear intent, and gaming' },
    personal: { name: 'Personal', details: 'Intentional non-work activity with a purpose: messaging friends and family, managing finances, booking travel, errands, life admin, and hobbies' },
  },
}
