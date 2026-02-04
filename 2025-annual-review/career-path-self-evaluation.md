A. Decision Ownership Map (critical)
	1.	Which of these do you currently decide?
	•	Data model boundaries
	•	API contracts across teams
	•	Consistency vs availability trade-offs
	•	Performance budgets
	•	Failure modes & rollback strategies
My answer: It’s mixed. I need to create technical plan, but also need to get the review and approval from my manager about those items.
	2.	Which do you influence but not decide?
	My answer: I may provide the estimation of cost for some production features, but I won’t be able to decide if we need to do it. So just technical consultant to PM.
	3.	Which are decided without you?
My answer:
Production-wise: what features to do.
Technology-wise: I only able to control the APIs of our team. I can’t control the infra team, the security team, the auth team, etc. Since we have a unified framework for all services in the same monorepo, it’s up to CTO to make the decision.

B. Blast Radius of Your Work

For the last 6–12 months:
	•	How many teams depend on systems you designed?
		My answer: I’m working on my team’s service which is a customer-facing service. So no other teams depend on the system I design. The APIs I designed are also be consumed directly by frontend.
	•	How many future features are constrained by your earlier decisions?
		My answer: At least half of the future features are constrained by my earlier decisions, since I designed the data representation and therefore the API designs.
	•	If you left tomorrow, what breaks vs what continues?
		My answer: since I’m working on APIs, hardly anything breaks since those APIs are running well. So everything continues

C. PM Interaction Pattern

Clarify:
	•	Do PMs come to you with:
		•	“Can we do this?”
		•	or “How do we implement this?”
My answer: usually, PM decides the major of the feature, and we provide technical consultation to see which functionalities should be done first. Usually, PM don’t ask about if it’s possible to do.
	•	Have you ever:
		•	Killed a feature on technical/business grounds?
		My answer: well, only a few times to compromise some features due to technical difficulties
		•	Reshaped a roadmap item significantly?
		My answer: no. Roadmap is not what I can impact

D. Time Allocation (very revealing)

Rough weekly breakdown:
	•	% coding
	•	% design docs
	•	% PM / cross-team discussions
	•	% debugging / ops / incidents

40% coding, 40% design docs, 10% discussions, 10% debugging/ops/incidents

E. Failure & Risk History

Tell me:
	•	A decision you made that later caused pain
	•	Whether you were accountable for the long-term consequences
My answer to the two questions: once I designed the technical plan for a project, I’m accountable for the long-term. Ever since the features have a question from customer success team or other team, they’ll ping me. It’s actually annoying since every time it happens I need to spend some time to investigate the specific data situation.

——
From the above situation, I’m diagnosed as Senior IC+ (Execution owner with local design authority), not staff.
Reasons are:
a technical authority inside the team, not across boundaries.
design but do not decide.
low organizational blast radius, even though your work is solid.
Staff-level ICs create absence pain. I create continuity
for long-term pain, I have responsibility without leverage

My problem: I’m absorbing the complexity, not removing it

Here are 3 ways forward:
be the canonical owner of a business domain. For example, the coaching service
be a cross-team service provider
work together with PM to validate features from technology perspective

