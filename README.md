# Chompy

![Chompy](http://38.media.tumblr.com/aa7029e25e94e31ca778b259729f6e79/tumblr_n9yvdfDSdG1sgl0ajo1_500.gif "Chompy!")

Software for a snack-dispensing motivator!

---

Chompy is AppEngine code along with Electric Imp code that can run a modified
Brookman Snackman to grant candy credits to users and allow using those credits
at will to dispense candy from the Snackman machine.

For example, it can be hooked up to git repos and grant credits to users when
they check in code!

## Setup & Installation

To use this, you must [first create a Snackbot](http://www.jameco.com/Jameco/workshop/JamecoBuilds/jamecobuilds-snackbot.html).

Once that is up and running:

    * Replace the agent and device code with the code in [/electricimp](/electricimp)
    * Create an app-engine app from this code.
        * You might want to customize the reward wording in [reward.go](/reward.go)
    * visit /config to initialize the app-engine app
        * You'll have to enter in the snackbot agent URL and a secret auth token used
          to ensure that only authorized people can grant rewards.

